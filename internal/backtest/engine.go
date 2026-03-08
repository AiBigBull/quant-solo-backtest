package backtest

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type RunInput struct {
	BarsBySymbol map[string][]Bar
	Symbols      []string
	Params       Params
	StartEquity  float64
}

func Run(input RunInput) (Result, error) {
	if len(input.Symbols) == 0 {
		return Result{}, fmt.Errorf("symbols cannot be empty")
	}
	if input.StartEquity <= 0 {
		return Result{}, fmt.Errorf("start equity must be positive")
	}
	for _, s := range input.Symbols {
		bars := input.BarsBySymbol[s]
		if len(bars) == 0 {
			return Result{}, fmt.Errorf("missing bars for %s", s)
		}
	}

	timeline := intersectTimeline(input.BarsBySymbol, input.Symbols)
	if len(timeline) < 300 {
		return Result{}, fmt.Errorf("insufficient aligned bars: %d", len(timeline))
	}

	closeSeries := map[string][]float64{}
	barLookup := map[string]map[int64]Bar{}
	for _, s := range input.Symbols {
		barLookup[s] = make(map[int64]Bar, len(input.BarsBySymbol[s]))
		for _, b := range input.BarsBySymbol[s] {
			barLookup[s][b.Time.Unix()] = b
		}
		series := make([]float64, len(timeline))
		for i, t := range timeline {
			series[i] = barLookup[s][t.Unix()].Close
		}
		closeSeries[s] = series
	}

	equity := input.StartEquity
	peak := equity
	maxDD := 0.0
	totalCost := 0.0
	totalTurnover := 0.0
	totalTrades := 0
	dailyEquivalentReturns := make([]float64, 0, len(timeline)-1)

	startIdx := maxInt(input.Params.SlowMA, maxInt(input.Params.MomentumLookback+1, input.Params.VolatilityLookback+2))
	if startIdx >= len(timeline)-1 {
		return Result{}, fmt.Errorf("insufficient bars after warmup, start index=%d timeline=%d", startIdx, len(timeline))
	}

	weightsPrev := make(map[string]float64, len(input.Symbols))
	monthly := map[string]float64{}
	monthStartEquity := equity
	currentMonth := timeline[startIdx].Format("2006-01")
	strategyStartTime := timeline[startIdx]
	strategyEndTime := timeline[len(timeline)-1]
	for i := startIdx; i < len(timeline)-1; i++ {
		t := timeline[i]
		monthKey := t.Format("2006-01")
		if monthKey != currentMonth {
			monthly[currentMonth] = equity/monthStartEquity - 1.0
			monthStartEquity = equity
			currentMonth = monthKey
		}

		targetWeights := computeWeights(input.Symbols, closeSeries, i, input.Params)
		turnover := 0.0
		for _, s := range input.Symbols {
			turnover += math.Abs(targetWeights[s] - weightsPrev[s])
		}
		cost := ((input.Params.FeeBps + input.Params.SlippageBps) / 10000.0) * turnover

		pnl := 0.0
		for _, s := range input.Symbols {
			ret := closeSeries[s][i+1]/closeSeries[s][i] - 1.0
			pnl += targetWeights[s] * ret
		}
		net := pnl - cost
		equity *= 1.0 + net
		days := timeline[i+1].Sub(timeline[i]).Hours() / 24.0
		if days <= 0 {
			days = 1
		}
		dailyEquivalent := math.Pow(1.0+net, 1.0/days) - 1.0

		if turnover > 0 {
			totalTrades++
		}
		totalTurnover += turnover
		totalCost += cost
		dailyEquivalentReturns = append(dailyEquivalentReturns, dailyEquivalent)

		if equity > peak {
			peak = equity
		}
		dd := 0.0
		if peak > 0 {
			dd = (peak - equity) / peak
		}
		if dd > maxDD {
			maxDD = dd
		}

		weightsPrev = targetWeights
	}

	monthly[currentMonth] = equity/monthStartEquity - 1.0
	profitableMonths, totalMonths := countProfitableMonths(monthly)
	strictMonthlyReturns := strictMonthlySeries(monthly)
	strictProfitable := 0
	for _, v := range strictMonthlyReturns {
		if v > 0 {
			strictProfitable++
		}
	}
	allMonthsProfitable := strictProfitable == len(strictMonthlyReturns) && len(strictMonthlyReturns) > 0
	maxConsecutiveLosing := maxConsecutiveLosingMonthlyReturns(strictMonthlyReturns)

	totalDays := strategyEndTime.Sub(strategyStartTime).Hours() / 24.0
	if totalDays <= 0 {
		totalDays = 1
	}
	annual := math.Pow(equity/input.StartEquity, 365.0/totalDays) - 1

	worstFullCalendarYear := computeWorstFullCalendarYearReturn(monthly)

	res := Result{
		Params:                      input.Params,
		InitialEquity:               input.StartEquity,
		FinalEquity:                 equity,
		CAGR:                        annual,
		AnnualizedReturn:            annual,
		MaxDrawdown:                 maxDD,
		Sharpe:                      annualizedSharpe(dailyEquivalentReturns),
		ProfitableMonths:            profitableMonths,
		TotalMonths:                 totalMonths,
		ProfitableMonthsRatio:       ratio(profitableMonths, totalMonths),
		AllMonthsProfitable:         allMonthsProfitable,
		MaxConsecutiveLosing:        maxConsecutiveLosing,
		StrictMonthsEvaluated:       len(strictMonthlyReturns),
		AvgDailyTurnover:            totalTurnover / math.Max(float64(len(dailyEquivalentReturns)), 1),
		TotalTrades:                 totalTrades,
		TransactionCostRatio:        totalCost / math.Max(float64(len(dailyEquivalentReturns)), 1),
		MonthlyReturns:              monthly,
		WorstFullCalendarYearReturn: worstFullCalendarYear,
	}
	return res, nil
}

// trendStrength converts the fast/slow MA spread into a bounded continuous signal in (-1, 1).
// Using tanh so small spreads produce proportionally small signals and large spreads saturate
// near ±1, eliminating the cliff-edge flip of the old binary ±1 approach.
// The scaling factor 10 makes the signal reach ~0.76 at a 10% spread and ~0.96 at a 20% spread.
func trendStrength(fast, slow float64) float64 {
	if slow <= 0 {
		return 0
	}
	spread := (fast - slow) / slow
	return math.Tanh(spread * 10.0)
}

// volPenalty returns a smooth multiplier in [0, 1] for the score based on realized volatility.
// Below 80% of the cap the multiplier is 1 (no penalty).
// Between 80% and 100% of the cap it ramps linearly from 1 down to 0.
// Above the cap it returns 0, fully suppressing the score.
func volPenalty(vol, cap float64) float64 {
	if cap <= 0 {
		return 0
	}
	rampStart := 0.8 * cap
	if vol <= rampStart {
		return 1.0
	}
	if vol >= cap {
		return 0.0
	}
	// linear ramp from 1 at rampStart to 0 at cap
	return (cap - vol) / (cap - rampStart)
}

func computeWeights(symbols []string, closes map[string][]float64, i int, p Params) map[string]float64 {
	scores := make(map[string]float64, len(symbols))
	gross := 0.0
	for _, s := range symbols {
		c := closes[s]
		fast := sma(c, i, p.FastMA)
		slow := sma(c, i, p.SlowMA)
		trend := trendStrength(fast, slow)

		mom := c[i]/c[i-p.MomentumLookback] - 1.0
		vol := realizedVol(c, i, p.VolatilityLookback)
		if vol <= 0 {
			vol = 1e-6
		}

		score := p.TrendWeight*trend + p.MomentumWeight*mom*10.0
		score *= volPenalty(vol, p.VolatilityCap)

		riskAdj := score / vol
		scores[s] = riskAdj
		gross += math.Abs(riskAdj)
	}

	weights := make(map[string]float64, len(symbols))
	if gross == 0 {
		for _, s := range symbols {
			weights[s] = 0
		}
		return weights
	}
	for _, s := range symbols {
		weights[s] = scores[s] / gross
	}
	return weights
}

func intersectTimeline(barsBySymbol map[string][]Bar, symbols []string) []time.Time {
	count := map[int64]int{}
	for _, s := range symbols {
		seen := map[int64]bool{}
		for _, b := range barsBySymbol[s] {
			ts := b.Time.Unix()
			if seen[ts] {
				continue
			}
			seen[ts] = true
			count[ts]++
		}
	}
	out := make([]int64, 0, len(count))
	for ts, c := range count {
		if c == len(symbols) {
			out = append(out, ts)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	timeline := make([]time.Time, len(out))
	for i, ts := range out {
		timeline[i] = time.Unix(ts, 0).UTC()
	}
	return timeline
}

func sma(series []float64, i int, period int) float64 {
	if period <= 1 {
		return series[i]
	}
	start := i - period + 1
	if start < 0 {
		start = 0
	}
	sum := 0.0
	count := 0
	for idx := start; idx <= i; idx++ {
		sum += series[idx]
		count++
	}
	if count == 0 {
		return series[i]
	}
	return sum / float64(count)
}

func realizedVol(series []float64, i int, lookback int) float64 {
	if lookback < 2 {
		lookback = 2
	}
	start := i - lookback + 1
	if start < 1 {
		start = 1
	}
	vals := make([]float64, 0, i-start+1)
	for idx := start; idx <= i; idx++ {
		r := series[idx]/series[idx-1] - 1.0
		vals = append(vals, r)
	}
	if len(vals) < 2 {
		return 1e-6
	}
	mean := 0.0
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	varSum := 0.0
	for _, v := range vals {
		d := v - mean
		varSum += d * d
	}
	variance := varSum / float64(len(vals)-1)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

func annualizedSharpe(rets []float64) float64 {
	if len(rets) < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	varSum := 0.0
	for _, r := range rets {
		d := r - mean
		varSum += d * d
	}
	variance := varSum / float64(len(rets)-1)
	if variance <= 0 {
		return 0
	}
	std := math.Sqrt(variance)
	return mean / std * math.Sqrt(365)
}

func countProfitableMonths(monthly map[string]float64) (int, int) {
	total := len(monthly)
	profitable := 0
	for _, v := range monthly {
		if v > 0 {
			profitable++
		}
	}
	return profitable, total
}

func strictMonthlySeries(monthly map[string]float64) []float64 {
	if len(monthly) == 0 {
		return nil
	}
	keys := make([]string, 0, len(monthly))
	for k := range monthly {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	start := 0
	end := len(keys)
	if len(keys) > 2 {
		start = 1
		end = len(keys) - 1
	}
	out := make([]float64, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, monthly[keys[i]])
	}
	return out
}

func maxConsecutiveLosingMonthlyReturns(monthlyReturns []float64) int {
	if len(monthlyReturns) == 0 {
		return 0
	}
	cur := 0
	maxLoss := 0
	for _, v := range monthlyReturns {
		if v < 0 {
			cur++
			if cur > maxLoss {
				maxLoss = cur
			}
		} else {
			cur = 0
		}
	}
	return maxLoss
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// computeWorstFullCalendarYearReturn calculates the worst compounded return
// across complete 12-month interior calendar years only.
// First and last months are excluded from this aggregation.
// Returns 0 if no full calendar year exists.
func computeWorstFullCalendarYearReturn(monthly map[string]float64) float64 {
	if len(monthly) == 0 {
		return 0
	}

	// Sort keys to identify interior months
	keys := make([]string, 0, len(monthly))
	for k := range monthly {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Exclude first and last month
	start := 0
	end := len(keys)
	if len(keys) > 2 {
		start = 1
		end = len(keys) - 1
	}

	if end-start < 12 {
		// Not enough interior months for a full calendar year
		return 0
	}

	// Group interior months by year
	yearReturns := map[string][]float64{}
	for i := start; i < end; i++ {
		monthKey := keys[i]
		year := monthKey[:4] // Extract YYYY from YYYY-MM
		yearReturns[year] = append(yearReturns[year], monthly[monthKey])
	}

	// Find worst full calendar year (all 12 months)
	worstReturn := 0.0
	foundFullYear := false
	for _, monthlyVals := range yearReturns {
		if len(monthlyVals) == 12 {
			// Compound the 12 monthly returns
			compounded := 1.0
			for _, ret := range monthlyVals {
				compounded *= (1.0 + ret)
			}
			yearReturn := compounded - 1.0
			if !foundFullYear || yearReturn < worstReturn {
				worstReturn = yearReturn
				foundFullYear = true
			}
		}
	}

	if !foundFullYear {
		return 0
	}
	return worstReturn
}
