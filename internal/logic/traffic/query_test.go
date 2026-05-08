package traffic

import "testing"

func TestNormalizeTrafficGroupBy(t *testing.T) {
	cases := map[string]string{
		"":            "day",
		"hour":        "hour",
		"gate":        "gate",
		"plateRegion": "plateregion",
		"inDir":       "indir",
		"bad":         "day",
	}
	for input, want := range cases {
		if got := normalizeTrafficGroupBy(input); got != want {
			t.Fatalf("normalizeTrafficGroupBy(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeQueryTime(t *testing.T) {
	if got := normalizeQueryTime("2026-05-08", false); got != "2026-05-08 00:00:00" {
		t.Fatalf("from date = %q", got)
	}
	if got := normalizeQueryTime("2026-05-08", true); got != "2026-05-08 23:59:59" {
		t.Fatalf("to date = %q", got)
	}
	if got := normalizeQueryTime("2026-05-08 10:20", false); got != "2026-05-08 10:20:00" {
		t.Fatalf("minute time = %q", got)
	}
}

func TestParseBoolFilter(t *testing.T) {
	if got, ok := parseBoolFilter("true"); !ok || !got {
		t.Fatalf("true parsed as %v, %v", got, ok)
	}
	if got, ok := parseBoolFilter("0"); !ok || got {
		t.Fatalf("0 parsed as %v, %v", got, ok)
	}
	if _, ok := parseBoolFilter(""); ok {
		t.Fatal("empty bool filter should not apply")
	}
}
