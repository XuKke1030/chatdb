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
