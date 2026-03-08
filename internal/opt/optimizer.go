package opt

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"quantsolo/internal/backtest"
	"quantsolo/internal/decision"
)

type Config struct {
	Iterations int
	Seed       int64
}

type Summary struct {
	BestOverall   backtest.Result
	BestDiscussed backtest.Result
	TotalRuns     int
	PassedRuns    int
}

type stressMetrics struct {
	FeeAnnualReturn       float64
	SlippageAnnualReturn  float64
	DataShockAnnualReturn float64
	WorstAnnualReturn     float64
	WorstDrawdown         float64
	NeighborhoodStd       float64
}

func Optimize(cfg Config, bars map[string][]backtest.Bar, symbols []string, startEquity float64) (Summary, error) {
	if cfg.Iterations <= 0 {
		return Summary{}, fmt.Errorf("iterations must be > 0")
	}
	seed := cfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))
	shockVariants := buildDataShockVariants(bars, seed, 3)
	seededParams := targetSeedParams()

	bestOverall := backtest.Result{OptimizationScore: -1e18}
	bestDiscussed := backtest.Result{OptimizationScore: -1e18}
	bestDiscussedDual := false
	passed := 0

	for i := 0; i < cfg.Iterations; i++ {
		p := paramsForIteration(i, seededParams, rng)
		res, err := backtest.Run(backtest.RunInput{
			BarsBySymbol: bars,
			Symbols:      symbols,
			Params:       p,
			StartEquity:  startEquity,
		})
		if err != nil {
			return Summary{}, err
		}

		shockBars := shockVariants[i%len(shockVariants)]
		stress, err := evaluateStressAndFragility(res, p, bars, shockBars, symbols, startEquity)
		if err != nil {
			return Summary{}, err
		}

		res = applyCommitteeOutcomeWithStress(res, stress)
		res.OptimizationScore = score(res, stress)

		if res.OptimizationScore > bestOverall.OptimizationScore {
			bestOverall = res
		}
		if res.CommitteePassed {
			passed++
			if isBetterDiscussed(res, bestDiscussed, bestDiscussedDual) {
				bestDiscussed = res
				bestDiscussedDual = meetsDualGoal(res)
			}
		}
	}

	if bestDiscussed.OptimizationScore <= -1e17 {
		bestDiscussed = bestOverall
		bestDiscussed.CommitteePassed = false
	}

	return Summary{
		BestOverall:   bestOverall,
		BestDiscussed: bestDiscussed,
		TotalRuns:     cfg.Iterations,
		PassedRuns:    passed,
	}, nil
}

func applyCommitteeOutcomeWithStress(res backtest.Result, stress stressMetrics) backtest.Result {
	res.StressWorstAnnualReturn = stress.WorstAnnualReturn
	res.StressWorstDrawdown = stress.WorstDrawdown
	res.StressNeighborhoodStd = stress.NeighborhoodStd

	outcome := decision.Evaluate(res)
	res.CommitteePassed = outcome.Passed
	res.CommitteeVotes = make(map[string]bool, len(outcome.Votes))
	res.CommitteeReasoning = make(map[string]string, len(outcome.Votes))
	for _, v := range outcome.Votes {
		res.CommitteeVotes[v.Name] = v.Pass
		res.CommitteeReasoning[v.Name] = v.Reason
	}

	return res
}

func targetSeedParams() []backtest.Params {
	return []backtest.Params{
		{
			FastMA:             26,
			SlowMA:             97,
			MomentumLookback:   36,
			VolatilityLookback: 47,
			TrendWeight:        1.200,
			MomentumWeight:     0.447,
			VolatilityCap:      0.0281,
			FeeBps:             6.42,
			SlippageBps:        1.12,
		},
		{
			FastMA:             24,
			SlowMA:             105,
			MomentumLookback:   8,
			VolatilityLookback: 53,
			TrendWeight:        1.714,
			MomentumWeight:     0.341,
			VolatilityCap:      0.0241,
			FeeBps:             7.64,
			SlippageBps:        6.05,
		},
		{
			FastMA:             18,
			SlowMA:             106,
			MomentumLookback:   11,
			VolatilityLookback: 45,
			TrendWeight:        0.58,
			MomentumWeight:     0.253,
			VolatilityCap:      0.0264,
			FeeBps:             9.85,
			SlippageBps:        6.07,
		},
		{
			FastMA:             22,
			SlowMA:             89,
			MomentumLookback:   11,
			VolatilityLookback: 54,
			TrendWeight:        1.612,
			MomentumWeight:     0.74,
			VolatilityCap:      0.0285,
			FeeBps:             2.65,
			SlippageBps:        5.21,
		},
	}
}

func paramsForIteration(i int, seeded []backtest.Params, rng *rand.Rand) backtest.Params {
	if i >= 0 && i < len(seeded) {
		return seeded[i]
	}
	return sampleParams(rng)
}

func sampleParams(rng *rand.Rand) backtest.Params {
	fast := rng.Intn(25) + 5
	slow := rng.Intn(80) + 30
	if slow <= fast+5 {
		slow = fast + 6
	}
	mom := rng.Intn(45) + 5
	vol := rng.Intn(45) + 10
	return backtest.Params{
		FastMA:             fast,
		SlowMA:             slow,
		MomentumLookback:   mom,
		VolatilityLookback: vol,
		TrendWeight:        0.5 + rng.Float64()*1.5,
		MomentumWeight:     0.2 + rng.Float64()*1.2,
		VolatilityCap:      0.02 + rng.Float64()*0.06,
		FeeBps:             2.0 + rng.Float64()*8.0,
		SlippageBps:        1.0 + rng.Float64()*8.0,
	}
}

func score(r backtest.Result, s stressMetrics) float64 {
	base := 0.0
	base += r.Sharpe * 36.0
	base += r.AnnualizedReturn * 70.0
	base -= r.MaxDrawdown * 45.0
	base += r.ProfitableMonthsRatio * 20.0
	base += boolScore(r.AllMonthsProfitable) * 8.0
	base -= float64(r.MaxConsecutiveLosing) * 3.0
	if r.AnnualizedReturn < 0.50 {
		base -= (0.50 - r.AnnualizedReturn) * 20.0
	}
	if r.WorstFullCalendarYearReturn < 0.40 {
		base -= (0.40 - r.WorstFullCalendarYearReturn) * 25.0
	}
	base -= r.TransactionCostRatio * 500.0

	base += s.WorstAnnualReturn * 55.0
	base -= s.WorstDrawdown * 30.0
	base -= s.NeighborhoodStd * 120.0

	if !r.CommitteePassed {
		base -= 1000.0
	}
	return base
}

func evaluateStressAndFragility(base backtest.Result, p backtest.Params, bars map[string][]backtest.Bar, shockBars map[string][]backtest.Bar, symbols []string, startEquity float64) (stressMetrics, error) {
	feeStress := p
	feeStress.FeeBps = p.FeeBps * 1.5

	slipStress := p
	slipStress.SlippageBps = p.SlippageBps + 5.0

	feeRes, err := backtest.Run(backtest.RunInput{BarsBySymbol: bars, Symbols: symbols, Params: feeStress, StartEquity: startEquity})
	if err != nil {
		return stressMetrics{}, err
	}
	slipRes, err := backtest.Run(backtest.RunInput{BarsBySymbol: bars, Symbols: symbols, Params: slipStress, StartEquity: startEquity})
	if err != nil {
		return stressMetrics{}, err
	}
	dataRes, err := backtest.Run(backtest.RunInput{BarsBySymbol: shockBars, Symbols: symbols, Params: p, StartEquity: startEquity})
	if err != nil {
		return stressMetrics{}, err
	}

	neighborStd, err := neighborhoodStd(p, bars, symbols, startEquity)
	if err != nil {
		return stressMetrics{}, err
	}

	worstAnnual := minFloat(base.AnnualizedReturn, minFloat(feeRes.AnnualizedReturn, minFloat(slipRes.AnnualizedReturn, dataRes.AnnualizedReturn)))
	worstDD := maxFloat(base.MaxDrawdown, maxFloat(feeRes.MaxDrawdown, maxFloat(slipRes.MaxDrawdown, dataRes.MaxDrawdown)))

	return stressMetrics{
		FeeAnnualReturn:       feeRes.AnnualizedReturn,
		SlippageAnnualReturn:  slipRes.AnnualizedReturn,
		DataShockAnnualReturn: dataRes.AnnualizedReturn,
		WorstAnnualReturn:     worstAnnual,
		WorstDrawdown:         worstDD,
		NeighborhoodStd:       neighborStd,
	}, nil
}

func neighborhoodStd(p backtest.Params, bars map[string][]backtest.Bar, symbols []string, startEquity float64) (float64, error) {
	neighbors := uniqueParams([]backtest.Params{
		p,
		withFastSlow(p, p.FastMA+2, p.SlowMA+3),
		withFastSlow(p, p.FastMA-2, p.SlowMA-3),
		withMomentum(p, p.MomentumLookback+3),
		withMomentum(p, p.MomentumLookback-3),
		withVolCap(p, p.VolatilityCap+0.005),
		withVolCap(p, p.VolatilityCap-0.005),
		withTrendWeight(p, p.TrendWeight+0.1),
		withTrendWeight(p, p.TrendWeight-0.1),
		withMomentumWeight(p, p.MomentumWeight+0.1),
		withMomentumWeight(p, p.MomentumWeight-0.1),
	})
	vals := make([]float64, 0, len(neighbors))
	for _, n := range neighbors {
		res, err := backtest.Run(backtest.RunInput{BarsBySymbol: bars, Symbols: symbols, Params: n, StartEquity: startEquity})
		if err != nil {
			return 0, err
		}
		vals = append(vals, res.AnnualizedReturn)
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
	return math.Sqrt(varSum / float64(len(vals))), nil
}

func withFastSlow(p backtest.Params, fast, slow int) backtest.Params {
	if fast < 3 {
		fast = 3
	}
	if slow < fast+6 {
		slow = fast + 6
	}
	p.FastMA = fast
	p.SlowMA = slow
	return p
}

func withMomentum(p backtest.Params, lookback int) backtest.Params {
	if lookback < 3 {
		lookback = 3
	}
	p.MomentumLookback = lookback
	return p
}

func withVolCap(p backtest.Params, cap float64) backtest.Params {
	if cap < 0.01 {
		cap = 0.01
	}
	if cap > 0.10 {
		cap = 0.10
	}
	p.VolatilityCap = cap
	return p
}

func withTrendWeight(p backtest.Params, w float64) backtest.Params {
	if w < 0.1 {
		w = 0.1
	}
	p.TrendWeight = w
	return p
}

func withMomentumWeight(p backtest.Params, w float64) backtest.Params {
	if w < 0.1 {
		w = 0.1
	}
	p.MomentumWeight = w
	return p
}

func uniqueParams(in []backtest.Params) []backtest.Params {
	seen := map[string]bool{}
	out := make([]backtest.Params, 0, len(in))
	for _, p := range in {
		k := strconv.Itoa(p.FastMA) + ":" + strconv.Itoa(p.SlowMA) + ":" + strconv.Itoa(p.MomentumLookback) + ":" + strconv.Itoa(p.VolatilityLookback) + ":" + fmt.Sprintf("%.6f", p.TrendWeight) + ":" + fmt.Sprintf("%.6f", p.MomentumWeight) + ":" + fmt.Sprintf("%.6f", p.VolatilityCap) + ":" + fmt.Sprintf("%.6f", p.FeeBps) + ":" + fmt.Sprintf("%.6f", p.SlippageBps)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

func buildDataShockVariants(bars map[string][]backtest.Bar, seed int64, count int) []map[string][]backtest.Bar {
	if count < 1 {
		count = 1
	}
	variants := make([]map[string][]backtest.Bar, 0, count)
	for i := 0; i < count; i++ {
		rng := rand.New(rand.NewSource(seed + int64(i+1)*7919))
		variant := make(map[string][]backtest.Bar, len(bars))
		for sym, arr := range bars {
			if len(arr) == 0 {
				variant[sym] = arr
				continue
			}
			staleProb := 0.04 + rng.Float64()*0.06
			phase := rng.Intn(7)
			shocked := make([]backtest.Bar, len(arr))
			copy(shocked, arr)
			for idx := 1; idx < len(shocked); idx++ {
				if idx%7 == phase || rng.Float64() < staleProb {
					prev := shocked[idx-1].Close
					shocked[idx].Open = prev
					shocked[idx].High = prev
					shocked[idx].Low = prev
					shocked[idx].Close = prev
				}
			}
			variant[sym] = shocked
		}
		variants = append(variants, variant)
	}
	return variants
}

func meetsDualGoal(r backtest.Result) bool {
	return r.AnnualizedReturn >= 0.50 && r.WorstFullCalendarYearReturn >= 0.40
}

func isBetterDiscussed(r, current backtest.Result, currentDual bool) bool {
	rDual := meetsDualGoal(r)
	if rDual && !currentDual {
		return true
	}
	if !rDual && currentDual {
		return false
	}
	return r.OptimizationScore > current.OptimizationScore
}

func boolScore(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
