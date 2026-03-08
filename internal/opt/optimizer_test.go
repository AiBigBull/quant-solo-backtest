package opt

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"quantsolo/internal/backtest"
)

func TestScoreOneSidedPenalty(t *testing.T) {
	base := backtest.Result{
		Sharpe:                0.0,
		AnnualizedReturn:      0.0,
		MaxDrawdown:           0.0,
		ProfitableMonthsRatio: 0.0,
		AllMonthsProfitable:   false,
		MaxConsecutiveLosing:  0,
		TransactionCostRatio:  0.0,
		CommitteePassed:       true,
	}
	stress := stressMetrics{}

	above := base
	above.AnnualizedReturn = 0.60

	below := base
	below.AnnualizedReturn = 0.40

	scoreAbove := score(above, stress)
	scoreBelow := score(below, stress)

	if scoreAbove <= scoreBelow {
		t.Errorf("60%% annualized (score=%.4f) should score higher than 40%% (score=%.4f)", scoreAbove, scoreBelow)
	}

	at50 := base
	at50.AnnualizedReturn = 0.50
	scoreAt50 := score(at50, stress)

	if scoreAbove < scoreAt50 {
		t.Errorf("60%% annualized (score=%.4f) should not be penalized below 50%% (score=%.4f)", scoreAbove, scoreAt50)
	}
}

func TestScoreSymmetryBroken(t *testing.T) {
	base := backtest.Result{CommitteePassed: true}
	stress := stressMetrics{}

	r60 := base
	r60.AnnualizedReturn = 0.60

	r40 := base
	r40.AnnualizedReturn = 0.40

	s60 := score(r60, stress)
	s40 := score(r40, stress)

	diff := s60 - s40
	if diff <= 0 {
		t.Errorf("60%% result should score higher than 40%% result; diff=%.4f", diff)
	}
}

func TestNeighborhoodIncludesContinuousWeights(t *testing.T) {
	p := backtest.Params{
		FastMA:             10,
		SlowMA:             30,
		MomentumLookback:   10,
		VolatilityLookback: 15,
		TrendWeight:        1.0,
		MomentumWeight:     0.8,
		VolatilityCap:      0.04,
		FeeBps:             4.0,
		SlippageBps:        3.0,
	}

	candidates := []backtest.Params{
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
	}

	foundTrendUp := false
	foundTrendDown := false
	foundMomUp := false
	foundMomDown := false

	for _, n := range candidates {
		if n.TrendWeight > p.TrendWeight {
			foundTrendUp = true
		}
		if n.TrendWeight < p.TrendWeight {
			foundTrendDown = true
		}
		if n.MomentumWeight > p.MomentumWeight {
			foundMomUp = true
		}
		if n.MomentumWeight < p.MomentumWeight {
			foundMomDown = true
		}
	}

	if !foundTrendUp || !foundTrendDown {
		t.Error("neighborhood should include TrendWeight perturbations in both directions")
	}
	if !foundMomUp || !foundMomDown {
		t.Error("neighborhood should include MomentumWeight perturbations in both directions")
	}
}

func TestWithTrendWeightClamp(t *testing.T) {
	p := backtest.Params{TrendWeight: 0.05}
	result := withTrendWeight(p, 0.0)
	if result.TrendWeight < 0.1 {
		t.Errorf("withTrendWeight should clamp to minimum 0.1, got %.4f", result.TrendWeight)
	}
}

func TestWithMomentumWeightClamp(t *testing.T) {
	p := backtest.Params{MomentumWeight: 0.05}
	result := withMomentumWeight(p, 0.0)
	if result.MomentumWeight < 0.1 {
		t.Errorf("withMomentumWeight should clamp to minimum 0.1, got %.4f", result.MomentumWeight)
	}
}

func TestParamsForIterationUsesSeededCandidatesFirst(t *testing.T) {
	seeded := targetSeedParams()
	rng := newTestRNG(42)

	for i := 0; i < len(seeded); i++ {
		got := paramsForIteration(i, seeded, rng)
		if got != seeded[i] {
			t.Fatalf("iteration %d should use seeded params", i)
		}
	}

	got := paramsForIteration(len(seeded), seeded, rng)
	for i := range seeded {
		if got == seeded[i] {
			t.Fatalf("iteration beyond seeded range should use random sample, got seeded[%d]=%#v", i, got)
		}
	}
}

func TestOptimizeWritesStressFieldsAndCommitteeLink(t *testing.T) {
	bars, symbols := syntheticBarsForOptimizeTest(360)

	summary, err := Optimize(Config{Iterations: 4, Seed: 7}, bars, symbols, 100000)
	if err != nil {
		t.Fatalf("optimize returned error: %v", err)
	}

	if summary.TotalRuns != 4 {
		t.Fatalf("expected total runs=4, got %d", summary.TotalRuns)
	}

	if math.IsNaN(summary.BestOverall.StressWorstAnnualReturn) || math.IsInf(summary.BestOverall.StressWorstAnnualReturn, 0) {
		t.Fatalf("invalid stress worst annual return: %v", summary.BestOverall.StressWorstAnnualReturn)
	}
	if math.IsNaN(summary.BestOverall.StressWorstDrawdown) || math.IsInf(summary.BestOverall.StressWorstDrawdown, 0) {
		t.Fatalf("invalid stress worst drawdown: %v", summary.BestOverall.StressWorstDrawdown)
	}
	if math.IsNaN(summary.BestOverall.StressNeighborhoodStd) || math.IsInf(summary.BestOverall.StressNeighborhoodStd, 0) {
		t.Fatalf("invalid stress neighborhood std: %v", summary.BestOverall.StressNeighborhoodStd)
	}
	if summary.BestOverall.StressWorstDrawdown < 0 {
		t.Fatalf("stress worst drawdown should be >=0, got %v", summary.BestOverall.StressWorstDrawdown)
	}

	if summary.PassedRuns == 0 {
		if summary.BestDiscussed.CommitteePassed {
			t.Fatalf("best discussed should be marked not passed when there are no passed runs")
		}
	} else if !summary.BestDiscussed.CommitteePassed {
		t.Fatalf("best discussed should be committee-passed when passed runs > 0")
	}
}

func TestApplyCommitteeOutcomeWithStressUsesStressBeforeEvaluation(t *testing.T) {
	base := backtest.Result{
		Sharpe:                1.3,
		AnnualizedReturn:      0.60,
		MaxDrawdown:           0.20,
		AvgDailyTurnover:      0.5,
		TransactionCostRatio:  0.001,
		ProfitableMonthsRatio: 0.70,
		MaxConsecutiveLosing:  1,
		TotalTrades:           50,
	}
	stress := stressMetrics{
		WorstAnnualReturn: -0.30,
		WorstDrawdown:     0.55,
		NeighborhoodStd:   0.02,
	}

	res := applyCommitteeOutcomeWithStress(base, stress)
	if res.CommitteePassed {
		t.Fatalf("expected committee rejection when stress limits are breached")
	}
	if res.CommitteeVotes["Risk Manager"] {
		t.Fatalf("expected Risk Manager vote to fail from stress gate")
	}
	if res.StressWorstAnnualReturn != stress.WorstAnnualReturn ||
		res.StressWorstDrawdown != stress.WorstDrawdown ||
		res.StressNeighborhoodStd != stress.NeighborhoodStd {
		t.Fatalf("stress fields should be written before committee evaluation")
	}
}

func TestMeetsDualGoal(t *testing.T) {
	cases := []struct {
		ann      float64
		worstYr  float64
		expected bool
	}{
		{0.55, 0.45, true},
		{0.50, 0.40, true},
		{0.49, 0.45, false},
		{0.55, 0.39, false},
		{0.49, 0.39, false},
	}
	for _, c := range cases {
		r := backtest.Result{AnnualizedReturn: c.ann, WorstFullCalendarYearReturn: c.worstYr}
		if got := meetsDualGoal(r); got != c.expected {
			t.Errorf("meetsDualGoal(ann=%.2f, worstYr=%.2f) = %v, want %v", c.ann, c.worstYr, got, c.expected)
		}
	}
}

func TestIsBetterDiscussedDualGoalBeatsHigherScore(t *testing.T) {
	highScore := backtest.Result{
		CommitteePassed:             true,
		AnnualizedReturn:            0.55,
		WorstFullCalendarYearReturn: 0.30,
		OptimizationScore:           100.0,
	}
	dualGoal := backtest.Result{
		CommitteePassed:             true,
		AnnualizedReturn:            0.52,
		WorstFullCalendarYearReturn: 0.42,
		OptimizationScore:           80.0,
	}

	if !isBetterDiscussed(dualGoal, highScore, false) {
		t.Error("dual-goal candidate should beat higher-score non-dual-goal candidate")
	}
	if isBetterDiscussed(highScore, dualGoal, true) {
		t.Error("non-dual-goal candidate should not beat dual-goal incumbent even with higher score")
	}
}

func TestIsBetterDiscussedSameTierUsesScore(t *testing.T) {
	lower := backtest.Result{
		CommitteePassed:             true,
		AnnualizedReturn:            0.52,
		WorstFullCalendarYearReturn: 0.42,
		OptimizationScore:           70.0,
	}
	higher := backtest.Result{
		CommitteePassed:             true,
		AnnualizedReturn:            0.55,
		WorstFullCalendarYearReturn: 0.45,
		OptimizationScore:           90.0,
	}

	if !isBetterDiscussed(higher, lower, true) {
		t.Error("within dual-goal tier, higher score should win")
	}
	if isBetterDiscussed(lower, higher, true) {
		t.Error("within dual-goal tier, lower score should not beat higher score")
	}

	nonDualLower := backtest.Result{OptimizationScore: 50.0, AnnualizedReturn: 0.45}
	nonDualHigher := backtest.Result{OptimizationScore: 60.0, AnnualizedReturn: 0.45}
	if !isBetterDiscussed(nonDualHigher, nonDualLower, false) {
		t.Error("within non-dual-goal tier, higher score should win")
	}
}

func syntheticBarsForOptimizeTest(n int) (map[string][]backtest.Bar, []string) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	bars := make(map[string][]backtest.Bar, len(symbols))
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for idx, s := range symbols {
		price := 100.0 + float64(idx)*10.0
		series := make([]backtest.Bar, 0, n)
		for i := 0; i < n; i++ {
			drift := 0.0012 + float64(idx)*0.0002
			wave := 0.0005 * math.Sin(float64(i+idx*7)/9.0)
			r := drift + wave
			next := price * (1.0 + r)
			high := math.Max(price, next) * 1.002
			low := math.Min(price, next) * 0.998
			series = append(series, backtest.Bar{
				Time:  start.AddDate(0, 0, i),
				Open:  price,
				High:  high,
				Low:   low,
				Close: next,
			})
			price = next
		}
		bars[s] = series
	}

	return bars, symbols
}

func newTestRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func TestScoreWorstFullCalendarYearReturn(t *testing.T) {
	base := backtest.Result{
		CommitteePassed:       true,
		AnnualizedReturn:      0.55,
		Sharpe:                1.0,
		MaxDrawdown:           0.10,
		ProfitableMonthsRatio: 0.70,
		TransactionCostRatio:  0.001,
	}
	stress := stressMetrics{}

	above := base
	above.WorstFullCalendarYearReturn = 0.50
	scoreAbove := score(above, stress)

	atTarget := base
	atTarget.WorstFullCalendarYearReturn = 0.40
	scoreAtTarget := score(atTarget, stress)

	below := base
	below.WorstFullCalendarYearReturn = 0.20
	scoreBelow := score(below, stress)

	if scoreAbove != scoreAtTarget {
		t.Errorf("above-target (%.4f) and at-target (%.4f) should score identically; got %.4f vs %.4f",
			above.WorstFullCalendarYearReturn, atTarget.WorstFullCalendarYearReturn, scoreAbove, scoreAtTarget)
	}

	if scoreBelow >= scoreAtTarget {
		t.Errorf("below-target worst year (score=%.4f) should score lower than at-target (score=%.4f)", scoreBelow, scoreAtTarget)
	}

	higher := base
	higher.WorstFullCalendarYearReturn = 0.30
	lower := base
	lower.WorstFullCalendarYearReturn = 0.10
	if score(higher, stress) <= score(lower, stress) {
		t.Errorf("higher worst-year return (0.30) should score higher than lower (0.10)")
	}
}
