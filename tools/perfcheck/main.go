package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type testCase struct {
	Name           string
	Method         string
	Path           string
	Body           string
	SSE            bool
	ExpectFastPath bool
	ExpectedIntent string
}

type caseResult struct {
	Name           string        `json:"name"`
	Requests       int64         `json:"requests"`
	Success        int64         `json:"success"`
	Errors         int64         `json:"errors"`
	AvgMs          float64       `json:"avgMs"`
	P50Ms          int64         `json:"p50Ms"`
	P95Ms          int64         `json:"p95Ms"`
	P99Ms          int64         `json:"p99Ms"`
	MinMs          int64         `json:"minMs"`
	MaxMs          int64         `json:"maxMs"`
	ErrorRate      float64       `json:"errorRate"`
	TargetP95      int64         `json:"targetP95"`
	TargetPass     bool          `json:"targetPass"`
	FastPathEvents int64         `json:"fastPathEvents,omitempty"`
	CacheHits      int64         `json:"cacheHits,omitempty"`
	Status         map[int]int64 `json:"status"`
}

func main() {
	baseURL := flag.String("base", "http://127.0.0.1:8000/api/v1", "base API URL")
	token := flag.String("token", "", "optional bearer token")
	concurrency := flag.Int("c", 8, "concurrency per case")
	duration := flag.Duration("d", 20*time.Second, "duration per case")
	includeLLM := flag.Bool("include-llm", false, "include LLM streaming qa/chats case")
	flag.Parse()

	cases := []testCase{
		{Name: "bootstrap", Method: http.MethodGet, Path: "/user/bootstrap"},
		{Name: "topics", Method: http.MethodGet, Path: "/topics"},
		{Name: "qa_retrieve", Method: http.MethodPost, Path: "/qa/retrieve", Body: `{"question":"权限如何控制","knowledgeCode":"","topK":5}`},
		{Name: "ask_number_traffic_top_gate_today", Method: http.MethodPost, Path: "/chats", Body: askNumberBody("哪个地方车流大？"), SSE: true, ExpectFastPath: true, ExpectedIntent: "traffic.top_gate.today"},
		{Name: "ask_number_traffic_gate_rank_today", Method: http.MethodPost, Path: "/chats", Body: askNumberBody("车流量排名前十的卡口"), SSE: true, ExpectFastPath: true, ExpectedIntent: "traffic.rank.gate"},
		{Name: "ask_number_traffic_trend_recent_days", Method: http.MethodPost, Path: "/chats", Body: askNumberBody("近几日车流有什么变化？"), SSE: true, ExpectFastPath: true, ExpectedIntent: "traffic.trend.recent_days"},
		{Name: "ask_number_traffic_hk_macau_ratio", Method: http.MethodPost, Path: "/chats", Body: askNumberBody("港澳车占比多少？"), SSE: true, ExpectFastPath: true, ExpectedIntent: "traffic.ratio.hk_macau"},
		{Name: "ask_number_traffic_weekend_workday", Method: http.MethodPost, Path: "/chats", Body: askNumberBody("周末和平日车流有什么变化？"), SSE: true, ExpectFastPath: true, ExpectedIntent: "traffic.compare.weekend_workday"},
		{Name: "ask_number_traffic_holiday_workday", Method: http.MethodPost, Path: "/chats", Body: askNumberBody("节假日车流和平日相比？"), SSE: true, ExpectFastPath: true, ExpectedIntent: "traffic.compare.holiday_workday"},
	}
	if *includeLLM {
		cases = append(cases, testCase{Name: "qa_chats_llm", Method: http.MethodPost, Path: "/qa/chats", Body: `{"message":"权限如何控制","knowledgeCode":"","topK":3,"ai":"deepseek","model":"deepseek-chat"}`, SSE: true})
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        1024,
			MaxIdleConnsPerHost: 1024,
			MaxConnsPerHost:     1024,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	results := make([]caseResult, 0, len(cases))
	for _, tc := range cases {
		results = append(results, runCase(client, *baseURL, *token, tc, *concurrency, *duration))
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(data))
}

func runCase(client *http.Client, baseURL string, token string, tc testCase, concurrency int, duration time.Duration) caseResult {
	var success int64
	var errorsCount int64
	var fastPathEvents int64
	var cacheHits int64
	var sumMs int64
	var latenciesMu sync.Mutex
	latencies := make([]int64, 0, concurrency*128)
	statusCounts := make(map[int]int64)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				elapsed, statusCode, ok, meta := doRequest(client, baseURL, token, tc)
				latenciesMu.Lock()
				latencies = append(latencies, elapsed)
				statusCounts[statusCode]++
				latenciesMu.Unlock()
				atomic.AddInt64(&sumMs, elapsed)
				if ok {
					atomic.AddInt64(&success, 1)
				} else {
					atomic.AddInt64(&errorsCount, 1)
				}
				if meta.FastPath {
					atomic.AddInt64(&fastPathEvents, 1)
				}
				if meta.CacheHit {
					atomic.AddInt64(&cacheHits, 1)
				}
			}
		}()
	}
	wg.Wait()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	total := int64(len(latencies))
	result := caseResult{
		Name:      tc.Name,
		Requests:  total,
		Success:   success,
		Errors:    errorsCount,
		TargetP95: targetP95(tc.Name),
		Status:    statusCounts,
	}
	result.FastPathEvents = fastPathEvents
	result.CacheHits = cacheHits
	if total == 0 {
		return result
	}
	result.MinMs = latencies[0]
	result.MaxMs = latencies[len(latencies)-1]
	result.AvgMs = float64(sumMs) / float64(total)
	result.P50Ms = percentile(latencies, 0.50)
	result.P95Ms = percentile(latencies, 0.95)
	result.P99Ms = percentile(latencies, 0.99)
	result.ErrorRate = float64(errorsCount) / float64(total)
	result.TargetPass = result.ErrorRate == 0 && result.P95Ms <= result.TargetP95
	return result
}

type responseMeta struct {
	FastPath bool
	CacheHit bool
}

func doRequest(client *http.Client, baseURL string, token string, tc testCase) (int64, int, bool, responseMeta) {
	var body io.Reader
	if tc.Body != "" {
		body = bytes.NewBufferString(tc.Body)
	}
	req, err := http.NewRequest(tc.Method, strings.TrimRight(baseURL, "/")+tc.Path, body)
	if err != nil {
		return 0, 0, false, responseMeta{}
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return elapsed, 0, false, responseMeta{}
	}
	defer resp.Body.Close()
	var meta responseMeta
	if tc.SSE {
		data, _ := io.ReadAll(resp.Body)
		meta = parseFastPathMeta(data, tc.ExpectedIntent)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	if tc.ExpectFastPath && !meta.FastPath {
		ok = false
	}
	return elapsed, resp.StatusCode, ok, meta
}

func percentile(items []int64, p float64) int64 {
	if len(items) == 0 {
		return 0
	}
	idx := int(float64(len(items)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(items) {
		idx = len(items) - 1
	}
	return items[idx]
}

func targetP95(name string) int64 {
	switch name {
	case "bootstrap", "topics":
		return 1500
	case "qa_retrieve":
		return 1000
	case "ask_number_traffic_top_gate_today",
		"ask_number_traffic_gate_rank_today",
		"ask_number_traffic_trend_recent_days",
		"ask_number_traffic_hk_macau_ratio",
		"ask_number_traffic_weekend_workday",
		"ask_number_traffic_holiday_workday":
		return 1000
	case "qa_chats_llm":
		return 8000
	default:
		return 3000
	}
}

func askNumberBody(message string) string {
	payload := map[string]any{
		"ai":         "deepseek",
		"model":      "deepseek-chat",
		"databaseId": 1,
		"topic":      "traffic",
		"message":    message,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func parseFastPathMeta(data []byte, expectedIntent string) responseMeta {
	text := string(data)
	meta := responseMeta{
		FastPath: strings.Contains(text, `"event":"fast_path"`) || strings.Contains(text, `"fastPath":true`),
		CacheHit: strings.Contains(text, `"cacheHit":true`),
	}
	if expectedIntent != "" && !strings.Contains(text, expectedIntent) {
		meta.FastPath = false
	}
	return meta
}
