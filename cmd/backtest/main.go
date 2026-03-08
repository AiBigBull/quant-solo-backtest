package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"quantsolo/internal/backtest"
	"quantsolo/internal/data"
	"quantsolo/internal/opt"
)

type report struct {
	GeneratedAt        time.Time       `json:"generated_at"`
	Source             string          `json:"source"`
	Symbols            []string        `json:"symbols"`
	Interval           string          `json:"interval"`
	Months             int             `json:"months"`
	Iterations         int             `json:"iterations"`
	CommitteePassRate  float64         `json:"committee_pass_rate"`
	SelectedFrom       string          `json:"selected_from"`
	BestDiscussed      backtest.Result `json:"best_discussed"`
	BestOverall        backtest.Result `json:"best_overall"`
	MeetsMonthlyGoal   bool            `json:"meets_monthly_goal"`
	MeetsStrictMonthly bool            `json:"meets_strict_monthly_goal"`
	MeetsAnnualGoal    bool            `json:"meets_annual_goal"`
	Conclusion         string          `json:"conclusion"`
}

func main() {
	baseURL := flag.String("source", data.DefaultBaseURL, "Binance data source")
	dataDir := flag.String("data-dir", "./data", "local data cache directory")
	symbolsFlag := flag.String("symbols", "BTCUSDT,ETHUSDT,SOLUSDT", "comma-separated symbols")
	interval := flag.String("interval", "1d", "kline interval")
	months := flag.Int("months", 24, "number of months to backtest")
	iterations := flag.Int("iterations", 100000, "optimizer iterations")
	startEquity := flag.Float64("start-equity", 100000, "initial capital")
	seed := flag.Int64("seed", 0, "random seed (0 means random)")
	noDownload := flag.Bool("no-download", false, "skip downloading and use cached zip files")
	flag.Parse()

	symbols := parseSymbols(*symbolsFlag)
	if len(symbols) == 0 {
		fatalf("no symbols configured")
	}

	client := data.NewBinanceClient(*baseURL)
	dataEndMonth := time.Now().UTC().AddDate(0, -1, 0)
	monthRange := data.BuildMonthlyRange(dataEndMonth, *months)
	barsBySymbol := make(map[string][]backtest.Bar, len(symbols))

	for _, symbol := range symbols {
		symbolDir := filepath.Join(*dataDir, symbol, *interval)
		zipFiles := make([]string, 0, len(monthRange))
		for _, m := range monthRange {
			name := fmt.Sprintf("%s-%s-%s.zip", symbol, *interval, m.Format("2006-01"))
			zipFiles = append(zipFiles, filepath.Join(symbolDir, name))
		}

		if !*noDownload {
			var err error
			zipFiles, err = client.DownloadMonthlyKlines(symbol, *interval, monthRange, symbolDir)
			if err != nil {
				fatalf("download failed for %s: %v", symbol, err)
			}
		}

		bars, err := client.LoadBarsFromZipFiles(zipFiles)
		if err != nil {
			fatalf("load bars failed for %s: %v", symbol, err)
		}
		from := monthRange[0]
		to := monthRange[len(monthRange)-1].AddDate(0, 1, 0).Add(-time.Nanosecond)
		bars = data.FilterBarsByTime(bars, from, to)
		if len(bars) < 300 {
			fatalf("insufficient bars for %s: %d", symbol, len(bars))
		}
		barsBySymbol[symbol] = bars
	}

	summary, err := opt.Optimize(opt.Config{
		Iterations: *iterations,
		Seed:       *seed,
	}, barsBySymbol, symbols, *startEquity)
	if err != nil {
		fatalf("optimize failed: %v", err)
	}

	best := summary.BestDiscussed
	selectedFrom := "best_discussed"
	if !best.CommitteePassed {
		best = summary.BestOverall
		selectedFrom = "best_overall_fallback"
	}
	passRate := float64(summary.PassedRuns) / float64(summary.TotalRuns)
	meetsMonthly := best.ProfitableMonthsRatio >= 0.5
	meetsStrictMonthly := best.AllMonthsProfitable
	meetsAnnual := best.AnnualizedReturn >= 0.50 && best.CommitteePassed
	meetsWorstFullYear := best.WorstFullCalendarYearReturn >= 0.40

	conclusion := "Selected strategy does not satisfy committee-approved 50% annual target; continue iteration with robustness constraints."
	if !best.CommitteePassed {
		conclusion = "No committee-approved strategy found; selected fallback is best overall for reference only."
	} else if meetsMonthly && meetsAnnual && meetsWorstFullYear {
		conclusion = "Best discussed strategy meets relaxed monthly profitability, approximately 50% annual return, and worst full calendar year targets."
	} else if meetsMonthly && meetsAnnual && !meetsWorstFullYear {
		conclusion = "Best discussed strategy meets relaxed monthly profitability and approximately 50% annual return targets, but worst full calendar year target not met."
	}
	if meetsStrictMonthly && meetsAnnual && meetsWorstFullYear {
		conclusion = "Best discussed strategy meets strict monthly profitability, approximately 50% annual return, and worst full calendar year targets."
	} else if meetsStrictMonthly && meetsAnnual && !meetsWorstFullYear {
		conclusion = "Best discussed strategy meets strict monthly profitability and approximately 50% annual return targets, but worst full calendar year target not met."
	}

	rep := report{
		GeneratedAt:        time.Now().UTC(),
		Source:             *baseURL,
		Symbols:            symbols,
		Interval:           *interval,
		Months:             *months,
		Iterations:         *iterations,
		CommitteePassRate:  passRate,
		SelectedFrom:       selectedFrom,
		BestDiscussed:      summary.BestDiscussed,
		BestOverall:        summary.BestOverall,
		MeetsMonthlyGoal:   meetsMonthly,
		MeetsStrictMonthly: meetsStrictMonthly,
		MeetsAnnualGoal:    meetsAnnual,
		Conclusion:         conclusion,
	}

	if err := os.MkdirAll("reports", 0o755); err != nil {
		fatalf("create reports dir failed: %v", err)
	}
	reportPath := filepath.Join("reports", "latest_report.json")
	if err := writeJSON(reportPath, rep); err != nil {
		fatalf("write report failed: %v", err)
	}

	printSummary(rep)
	fmt.Printf("Report written: %s\n", reportPath)
}

func parseSymbols(input string) []string {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(strings.ToUpper(p))
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func printSummary(r report) {
	best := r.BestDiscussed
	if !best.CommitteePassed {
		best = r.BestOverall
	}
	fmt.Println("===== BTC/ETH/SOL 1x Futures Mixed Backtest =====")
	fmt.Printf("Source: %s\n", r.Source)
	fmt.Printf("Symbols: %s\n", strings.Join(r.Symbols, ","))
	fmt.Printf("Iterations: %d | Committee pass rate: %.2f%%\n", r.Iterations, r.CommitteePassRate*100)
	fmt.Printf("Annualized Return: %.2f%% | Sharpe: %.2f | MaxDD: %.2f%%\n", best.AnnualizedReturn*100, best.Sharpe, best.MaxDrawdown*100)
	fmt.Printf("Worst Full Calendar Year Return: %.2f%%\n", best.WorstFullCalendarYearReturn*100)
	fmt.Printf("Profitable Months: %d/%d (%.2f%%)\n", best.ProfitableMonths, best.TotalMonths, best.ProfitableMonthsRatio*100)
	fmt.Printf("All months profitable (strict core %d months): %t | Max consecutive losing months: %d\n", best.StrictMonthsEvaluated, best.AllMonthsProfitable, best.MaxConsecutiveLosing)
	fmt.Printf("Monthly goal met: %t | Strict monthly goal met: %t | Annual 50%% target met: %t\n", r.MeetsMonthlyGoal, r.MeetsStrictMonthly, r.MeetsAnnualGoal)
	fmt.Printf("Conclusion: %s\n", r.Conclusion)
	fmt.Printf("Best Params: fast=%d slow=%d mom=%d vol=%d trendW=%.3f momW=%.3f volCap=%.4f feeBps=%.2f slippageBps=%.2f\n",
		best.Params.FastMA,
		best.Params.SlowMA,
		best.Params.MomentumLookback,
		best.Params.VolatilityLookback,
		best.Params.TrendWeight,
		best.Params.MomentumWeight,
		best.Params.VolatilityCap,
		best.Params.FeeBps,
		best.Params.SlippageBps,
	)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
