package model

type PopulationAggregateQuery struct {
	DateFrom string
	DateTo   string
	Region   string
	GridName string
	GroupBy  string // "hour", "day", "region"
}

type PopulationAggregateSummary struct {
	TotalInCount            int `json:"totalInCount"`
	TotalOutCount           int `json:"totalOutCount"`
	TotalNetInCount         int `json:"totalNetInCount"`
	TotalFloatingPopulation int `json:"totalFloatingPopulation"`
}

type PopulationAggregateSeriesItem struct {
	Name               string `json:"name"`
	InCount            int    `json:"inCount"`
	OutCount           int    `json:"outCount"`
	NetInCount         int    `json:"netInCount"`
	FloatingPopulation int    `json:"floatingPopulation"`
}

type PopulationAggregateResult struct {
	Summary PopulationAggregateSummary      `json:"summary"`
	Series  []PopulationAggregateSeriesItem `json:"series"`
	GroupBy string                          `json:"groupBy"`
}

type PopulationYoYItem struct {
	Name       string  `json:"name"`
	CurrentVal int     `json:"currentVal"`
	PriorVal   int     `json:"priorVal"`
	ChangePct  float64 `json:"changePct"`
}

type TagDistributionQuery struct {
	DateFrom string   // Y-m-d
	DateTo   string   // Y-m-d
	AllDates bool     // true时不加日期限制
	Tag      string   // 年龄/性别/省内城市来源/省外城市来源/省外来源
	Type     int      // 0=不区分, 1=总, 2=进, 3=出
	Area     string   // 区域筛选，空=全部
	Labels   []string // 指定标签值筛选，空=全部
}

type TagDistributionItem struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Pct   string `json:"pct"`
}

type TagDistributionResult struct {
	Tag      string                `json:"tag"`
	Type     int                   `json:"type"`
	Total    int                   `json:"total"`
	Items    []TagDistributionItem `json:"items"`
}
