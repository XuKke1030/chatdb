package service

import (
	"ai-chat-sql/internal/model"
	"context"
)

type (
	IPopulation interface {
		InitTables(ctx context.Context) error
		RefreshAggregates(ctx context.Context, dateFrom, dateTo string) error
		Aggregate(ctx context.Context, query model.PopulationAggregateQuery) (*model.PopulationAggregateResult, error)
		YoYCompare(ctx context.Context, dateFrom, dateTo, groupBy string) ([]model.PopulationYoYItem, error)
	}
)

var localPopulation IPopulation

func Population() IPopulation {
	if localPopulation == nil {
		panic("implement not found for interface IPopulation, forgot register?")
	}
	return localPopulation
}

func RegisterPopulation(i IPopulation) {
	localPopulation = i
}
