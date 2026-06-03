package model

type PopulationAggregateQuery struct {
	DateFrom string `json:"dateFrom" dc:"起始日期 Y-m-d"`
	DateTo   string `json:"dateTo"   dc:"截止日期 Y-m-d"`
	Region   string `json:"region"   dc:"区域筛选"`
	GridName string `json:"gridName" dc:"网格筛选"`
	GroupBy  string `json:"groupBy"  dc:"分组方式：hour/day/region"`
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
	DateFrom string   `json:"dateFrom" dc:"起始日期 Y-m-d"`
	DateTo   string   `json:"dateTo"   dc:"截止日期 Y-m-d"`
	AllDates bool     `json:"allDates"  dc:"true时不加日期限制"`
	Tag      string   `json:"tag"       dc:"标签类别"`
	Type     int      `json:"type"      dc:"0=不区分, 1=总, 2=进, 3=出"`
	Area     string   `json:"area"      dc:"区域筛选"`
	Labels   []string `json:"labels"    dc:"指定标签值筛选"`
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
