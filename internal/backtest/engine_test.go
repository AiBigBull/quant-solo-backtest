package backtest

import (
	"math"
	"testing"
)

// TestTrendStrengthContinuous verifies that small changes in fast/slow spread
// produce proportionally small changes in signal (no cliff-edge ±1 flip).
func TestTrendStrengthContinuous(t *testing.T) {
	slow := 100.0

	// tiny positive spread should give a small positive signal, not jump to +1
	tiny := trendStrength(100.001, slow)
	if tiny <= 0 {
		t.Fatalf("expected positive signal for fast > slow, got %v", tiny)
	}
	if tiny >= 0.1 {
		t.Fatalf("expected tiny signal for 0.001%% spread, got %v", tiny)
	}

	// moderate spread should give a moderate signal
	moderate := trendStrength(101.0, slow) // 1% spread
	if moderate <= 0.09 || moderate >= 0.5 {
		t.Fatalf("expected moderate signal ~0.1 for 1%% spread, got %v", moderate)
	}

	// large spread should saturate near 1 but not exceed it
	large := trendStrength(120.0, slow) // 20% spread
	if large <= 0.9 || large >= 1.0 {
		t.Fatalf("expected signal near 1 for 20%% spread, got %v", large)
	}

	// signal should be monotonically increasing with spread
	s1 := trendStrength(100.5, slow)
	s2 := trendStrength(101.0, slow)
	s3 := trendStrength(102.0, slow)
	if !(s1 < s2 && s2 < s3) {
		t.Fatalf("signal not monotonically increasing: %v %v %v", s1, s2, s3)
	}

	// negative spread should give negative signal (symmetric)
	neg := trendStrength(99.0, slow)
	pos := trendStrength(101.0, slow)
	if math.Abs(neg+pos) > 1e-9 {
		t.Fatalf("expected symmetric signal, got neg=%v pos=%v", neg, pos)
	}
}

// TestVolPenaltySmoothRamp verifies that the volatility penalty decays smoothly
// near the cap rather than jumping from 1 to 0 at a single threshold.
func TestVolPenaltySmoothRamp(t *testing.T) {
	cap := 0.05

	// well below ramp start (80% of cap = 0.04): no penalty
	if p := volPenalty(0.03, cap); p != 1.0 {
		t.Fatalf("expected penalty=1 below ramp, got %v", p)
	}

	// at ramp start: still 1
	if p := volPenalty(0.04, cap); p != 1.0 {
		t.Fatalf("expected penalty=1 at ramp start, got %v", p)
	}

	// midpoint of ramp (0.045): should be ~0.5
	mid := volPenalty(0.045, cap)
	if math.Abs(mid-0.5) > 1e-9 {
		t.Fatalf("expected penalty=0.5 at ramp midpoint, got %v", mid)
	}

	// at cap: fully suppressed
	if p := volPenalty(cap, cap); p != 0.0 {
		t.Fatalf("expected penalty=0 at cap, got %v", p)
	}

	// above cap: fully suppressed
	if p := volPenalty(0.10, cap); p != 0.0 {
		t.Fatalf("expected penalty=0 above cap, got %v", p)
	}

	// penalty is monotonically decreasing through the ramp
	p1 := volPenalty(0.041, cap)
	p2 := volPenalty(0.045, cap)
	p3 := volPenalty(0.049, cap)
	if !(p1 > p2 && p2 > p3) {
		t.Fatalf("penalty not monotonically decreasing: %v %v %v", p1, p2, p3)
	}
}

// TestCalendarYearOneFullYear verifies that a single full calendar year is selected correctly.
func TestCalendarYearOneFullYear(t *testing.T) {
	monthly := map[string]float64{
		// Partial first year (excluded)
		"2023-12": 0.05,
		// Full year 2024
		"2024-01": 0.01,
		"2024-02": 0.02,
		"2024-03": 0.01,
		"2024-04": 0.02,
		"2024-05": 0.01,
		"2024-06": 0.02,
		"2024-07": 0.01,
		"2024-08": 0.02,
		"2024-09": 0.01,
		"2024-10": 0.02,
		"2024-11": 0.01,
		"2024-12": 0.02,
		// Partial last year (excluded)
		"2025-01": 0.05,
	}
	worst := computeWorstFullCalendarYearReturn(monthly)
	// Expected: (1.01 * 1.02 * 1.01 * 1.02 * 1.01 * 1.02 * 1.01 * 1.02 * 1.01 * 1.02 * 1.01 * 1.02) - 1
	// = (1.01 * 1.02)^6 - 1 ≈ 1.0302^6 - 1 ≈ 0.1956
	if worst < 0.19 || worst > 0.20 {
		t.Fatalf("expected worst ~0.1956 for single full year, got %v", worst)
	}
}

// TestCalendarYearTwoFullYears verifies that the worse of two full years is selected.
func TestCalendarYearTwoFullYears(t *testing.T) {
	monthly := map[string]float64{
		// Partial first year (excluded)
		"2023-12": 0.05,
		// Full year 2024: good year
		"2024-01": 0.02,
		"2024-02": 0.02,
		"2024-03": 0.02,
		"2024-04": 0.02,
		"2024-05": 0.02,
		"2024-06": 0.02,
		"2024-07": 0.02,
		"2024-08": 0.02,
		"2024-09": 0.02,
		"2024-10": 0.02,
		"2024-11": 0.02,
		"2024-12": 0.02,
		// Full year 2025: bad year
		"2025-01": -0.05,
		"2025-02": -0.05,
		"2025-03": -0.05,
		"2025-04": -0.05,
		"2025-05": -0.05,
		"2025-06": -0.05,
		"2025-07": -0.05,
		"2025-08": -0.05,
		"2025-09": -0.05,
		"2025-10": -0.05,
		"2025-11": -0.05,
		"2025-12": -0.05,
		// Partial last year (excluded)
		"2026-01": 0.05,
	}
	worst := computeWorstFullCalendarYearReturn(monthly)
	// 2024: (1.02)^12 - 1 ≈ 0.2682
	// 2025: (0.95)^12 - 1 ≈ -0.4596
	// Worst should be 2025 ≈ -0.4596
	if worst > -0.45 || worst < -0.47 {
		t.Fatalf("expected worst ~-0.4596 for two full years, got %v", worst)
	}
}

// TestCalendarYearNoFullYear verifies that 0 is returned when no full calendar year exists.
func TestCalendarYearNoFullYear(t *testing.T) {
	monthly := map[string]float64{
		"2024-06": 0.01,
		"2024-07": 0.02,
		"2024-08": 0.01,
		"2024-09": 0.02,
		"2024-10": 0.01,
		"2024-11": 0.02,
	}
	worst := computeWorstFullCalendarYearReturn(monthly)
	if worst != 0 {
		t.Fatalf("expected 0 for no full year, got %v", worst)
	}
}

// TestComputeWeightsGrossNormalization verifies sum(abs(weight)) <= 1 is preserved.
func TestComputeWeightsGrossNormalization(t *testing.T) {
	n := 400
	series := make([]float64, n)
	for i := range series {
		series[i] = 100.0 + float64(i)*0.1
	}
	closes := map[string][]float64{
		"A": series,
		"B": series,
		"C": series,
	}
	p := Params{
		FastMA:             10,
		SlowMA:             30,
		MomentumLookback:   20,
		VolatilityLookback: 20,
		TrendWeight:        1.0,
		MomentumWeight:     0.5,
		VolatilityCap:      0.05,
	}
	symbols := []string{"A", "B", "C"}
	for i := 50; i < n; i++ {
		w := computeWeights(symbols, closes, i, p)
		gross := 0.0
		for _, s := range symbols {
			gross += math.Abs(w[s])
		}
		if gross > 1.0+1e-9 {
			t.Fatalf("gross exposure %v > 1 at bar %d", gross, i)
		}
	}
}
