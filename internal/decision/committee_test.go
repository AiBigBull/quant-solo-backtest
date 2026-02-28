package decision

import (
	"testing"

	"quantsolo/internal/backtest"
)

func TestEvaluatePass(t *testing.T) {
	res := backtest.Result{
		Sharpe:                1.2,
		AnnualizedReturn:      0.55,
		MaxDrawdown:           0.2,
		AvgDailyTurnover:      0.8,
		TransactionCostRatio:  0.001,
		ProfitableMonthsRatio: 0.7,
		TotalTrades:           100,
	}
	out := Evaluate(res)
	if !out.Passed {
		t.Fatalf("expected passed outcome")
	}
}

func TestEvaluateReject(t *testing.T) {
	res := backtest.Result{
		Sharpe:                0.2,
		AnnualizedReturn:      0.1,
		MaxDrawdown:           0.5,
		AvgDailyTurnover:      3.0,
		TransactionCostRatio:  0.01,
		ProfitableMonthsRatio: 0.2,
		TotalTrades:           5,
	}
	out := Evaluate(res)
	if out.Passed {
		t.Fatalf("expected rejected outcome")
	}
}
