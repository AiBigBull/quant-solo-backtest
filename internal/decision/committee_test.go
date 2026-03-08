package decision

import (
	"testing"

	"quantsolo/internal/backtest"
)

// TestEvaluatePass verifies a robust strategy meeting all raised thresholds passes.
func TestEvaluatePass(t *testing.T) {
	res := backtest.Result{
		Sharpe:                  1.2,
		AnnualizedReturn:        0.55,
		MaxDrawdown:             0.2,
		AvgDailyTurnover:        0.8,
		TransactionCostRatio:    0.001,
		ProfitableMonthsRatio:   0.7,
		MaxConsecutiveLosing:    1,
		TotalTrades:             100,
		StressWorstDrawdown:     0.30,
		StressWorstAnnualReturn: 0.05,
	}
	out := Evaluate(res)
	if !out.Passed {
		t.Fatalf("expected passed outcome, votes: %+v", out.Votes)
	}
}

// TestEvaluateReject verifies a weak strategy is rejected.
func TestEvaluateReject(t *testing.T) {
	res := backtest.Result{
		Sharpe:                  0.2,
		AnnualizedReturn:        0.1,
		MaxDrawdown:             0.5,
		AvgDailyTurnover:        3.0,
		TransactionCostRatio:    0.01,
		ProfitableMonthsRatio:   0.2,
		MaxConsecutiveLosing:    5,
		TotalTrades:             5,
		StressWorstDrawdown:     0.60,
		StressWorstAnnualReturn: -0.40,
	}
	out := Evaluate(res)
	if out.Passed {
		t.Fatalf("expected rejected outcome, votes: %+v", out.Votes)
	}
}

// TestEvaluateStressReject verifies a strategy with strong base metrics but
// poor stress performance is rejected by the risk vote.
func TestEvaluateStressReject(t *testing.T) {
	res := backtest.Result{
		Sharpe:                1.5,
		AnnualizedReturn:      0.60,
		MaxDrawdown:           0.18,
		AvgDailyTurnover:      0.5,
		TransactionCostRatio:  0.001,
		ProfitableMonthsRatio: 0.75,
		MaxConsecutiveLosing:  1,
		TotalTrades:           120,
		// Stress scenario: drawdown blows out under fee/slippage shock
		StressWorstDrawdown:     0.55,
		StressWorstAnnualReturn: -0.30,
	}
	out := Evaluate(res)
	if out.Passed {
		t.Fatalf("expected stress-based rejection, votes: %+v", out.Votes)
	}
	// Confirm it is the risk vote that failed
	for _, v := range out.Votes {
		if v.Name == "Risk Manager" && v.Pass {
			t.Fatalf("expected Risk Manager to reject, but it passed")
		}
	}
}
