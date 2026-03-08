package backtest

import "time"

type Bar struct {
	Time  time.Time
	Open  float64
	High  float64
	Low   float64
	Close float64
}

type Params struct {
	FastMA             int
	SlowMA             int
	MomentumLookback   int
	VolatilityLookback int
	TrendWeight        float64
	MomentumWeight     float64
	VolatilityCap      float64
	FeeBps             float64
	SlippageBps        float64
}

type Result struct {
	Params                      Params
	InitialEquity               float64
	FinalEquity                 float64
	CAGR                        float64
	AnnualizedReturn            float64
	MaxDrawdown                 float64
	Sharpe                      float64
	ProfitableMonths            int
	TotalMonths                 int
	ProfitableMonthsRatio       float64
	AllMonthsProfitable         bool
	MaxConsecutiveLosing        int
	StrictMonthsEvaluated       int
	AvgDailyTurnover            float64
	TotalTrades                 int
	TransactionCostRatio        float64
	MonthlyReturns              map[string]float64
	CommitteePassed             bool
	CommitteeVotes              map[string]bool
	CommitteeReasoning          map[string]string
	OptimizationScore           float64
	StressWorstAnnualReturn     float64 `json:"stress_worst_annual_return"`
	StressWorstDrawdown         float64 `json:"stress_worst_drawdown"`
	StressNeighborhoodStd       float64 `json:"stress_neighborhood_std"`
	WorstFullCalendarYearReturn float64 `json:"worst_full_calendar_year_return"`
}
