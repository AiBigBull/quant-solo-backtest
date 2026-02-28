package decision

import "quantsolo/internal/backtest"

type Vote struct {
	Name   string
	Pass   bool
	Reason string
}

type Outcome struct {
	Passed bool
	Votes  []Vote
}

func Evaluate(res backtest.Result) Outcome {
	votes := []Vote{
		researchVote(res),
		riskVote(res),
		executionVote(res),
		productVote(res),
		devVote(res),
	}
	passed := true
	for _, v := range votes {
		if !v.Pass {
			passed = false
			break
		}
	}
	return Outcome{Passed: passed, Votes: votes}
}

func researchVote(res backtest.Result) Vote {
	pass := res.Sharpe >= 0.8 && res.AnnualizedReturn >= 0.20
	reason := "Sharpe>=0.8 and annual return>=20%"
	if !pass {
		reason = "alpha robustness below threshold"
	}
	return Vote{Name: "Quant Research", Pass: pass, Reason: reason}
}

func riskVote(res backtest.Result) Vote {
	pass := res.MaxDrawdown <= 0.25
	reason := "max drawdown <=25%"
	if !pass {
		reason = "drawdown too high"
	}
	return Vote{Name: "Risk Manager", Pass: pass, Reason: reason}
}

func executionVote(res backtest.Result) Vote {
	pass := res.AvgDailyTurnover <= 1.5 && res.TransactionCostRatio <= 0.003
	reason := "turnover and cost within execution budget"
	if !pass {
		reason = "execution cost/turnover too high"
	}
	return Vote{Name: "Execution", Pass: pass, Reason: reason}
}

func productVote(res backtest.Result) Vote {
	pass := res.ProfitableMonthsRatio >= 0.55 && res.MaxConsecutiveLosing <= 2
	reason := "monthly profitability ratio >=55% and max consecutive losing months <=2"
	if !pass {
		reason = "monthly profitability consistency too weak"
	}
	return Vote{Name: "Product Manager", Pass: pass, Reason: reason}
}

func devVote(res backtest.Result) Vote {
	pass := res.TotalTrades >= 20
	reason := "strategy active enough to monitor and maintain"
	if !pass {
		reason = "too few trades; low observability"
	}
	return Vote{Name: "Quant Dev", Pass: pass, Reason: reason}
}
