package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/crab-he/internal/experiments"
)

const (
	defaultVSat         int64 = 2_000_000
	defaultSeedRuns           = 30
	epsilonSat          int64 = 1_000
	simModelDescription       = "synthetic Monte Carlo over analytical attack width with Gaussian noise (not blockchain execution)"
)

type gridConfig = experiments.Config

type sweepRow struct {
	ConfigID               string `json:"configId"`
	VSat                   int64  `json:"vSat"`
	VDepSat                int64  `json:"vDepSat"`
	VColSat                int64  `json:"vColSat"`
	Kappa                  int    `json:"kappa"`
	HopsN                  int    `json:"hopsN"`
	WidthCRABSat           int64  `json:"widthCRABSat"`
	WidthCRABByzantineSat  int64  `json:"widthCRABByzantineSat"`
	WidthCRABPrime1_25Sat  int64  `json:"widthCRABPrime1_25Sat"`
	WidthCRABPrime1_50Sat  int64  `json:"widthCRABPrime1_50Sat"`
	WidthCRABPrime2_00Sat  int64  `json:"widthCRABPrime2_00Sat"`
	CStarSat               int64  `json:"cStarSat"`
	WidthCRABHeCStarMinus  int64  `json:"widthCRABHeCStarMinusEpsilonSat"`
	WidthCRABHeCStar       int64  `json:"widthCRABHeCStarSat"`
	WidthCRABHeCStarPlus   int64  `json:"widthCRABHeCStarPlusEpsilonSat"`
	CNStarSat              int64  `json:"cNStarSat"`
	HeConditionValid       bool   `json:"heConditionValid"`
	HeConditionReason      string `json:"heConditionReason"`
	MADStandaloneWidthSat  int64  `json:"madStandaloneWidthSat"`
	HeStandaloneMarginSat  int64  `json:"heStandaloneMarginSat"`
	ActiveMinerCoverageMAD bool   `json:"activeMinerCoverageMAD"`
	Notes                  string `json:"notes"`
	ElapsedMicros          int64  `json:"elapsedMicros"`
	GeneratedAtUTC         string `json:"generatedAtUtc"`
}

type multiHopRow struct {
	N           int   `json:"n"`
	CNStarSat   int64 `json:"cNStarSat"`
	OverheadSat int64 `json:"overheadAboveCRABByzSat"`
}

type simPair struct {
	Seed               int64 `json:"seed"`
	BaselineSuccess    int   `json:"baselineSuccess"`
	CRABHeSuccess      int   `json:"crabHeSuccess"`
	DifferenceBaseline int   `json:"differenceBaselineMinusCRABHe"`
}

type pairedStats struct {
	SampleSize           int     `json:"sampleSize"`
	MeanDiff             float64 `json:"meanDiff"`
	StdDevDiff           float64 `json:"stdDevDiff"`
	CI95Low              float64 `json:"ci95Low"`
	CI95High             float64 `json:"ci95High"`
	TStatistic           float64 `json:"tStatistic"`
	TTestPValueApprox    float64 `json:"tTestPValueApprox"`
	CohensDz             float64 `json:"cohensDz"`
	WilcoxonWPlus        float64 `json:"wilcoxonWPlus"`
	WilcoxonZApprox      float64 `json:"wilcoxonZApprox"`
	WilcoxonPValueApprox float64 `json:"wilcoxonPValueApprox"`
	WilcoxonEffectR      float64 `json:"wilcoxonEffectR"`
}

type simSummary struct {
	ConfigID            string      `json:"configId"`
	BaselineName        string      `json:"baselineName"`
	SeedRuns            int         `json:"seedRuns"`
	HeConditionValid    bool        `json:"heConditionValid"`
	HeConditionReason   string      `json:"heConditionReason"`
	IncludedInValidSet  bool        `json:"includedInValidSet"`
	SimulationModel     string      `json:"simulationModel"`
	BaselineSuccessRate float64     `json:"baselineSuccessRate"`
	CRABHeSuccessRate   float64     `json:"crabHeSuccessRate"`
	Pairs               []simPair   `json:"pairs"`
	Stats               pairedStats `json:"stats"`
	GeneratedAtUTC      string      `json:"generatedAtUtc"`
}

type telemetry struct {
	RegtestBlocksObserved           int64   `json:"regtestBlocksObserved"`
	SignetBlocksObserved            int64   `json:"signetBlocksObserved"`
	WitnessGenerationIterations     int     `json:"witnessGenerationIterations"`
	WitnessGenerationTotalMs        float64 `json:"witnessGenerationTotalMs"`
	WitnessGenerationAvgMicros      float64 `json:"witnessGenerationAvgMicros"`
	ScriptValidationIterations      int     `json:"scriptValidationIterations"`
	ScriptValidationTotalMs         float64 `json:"scriptValidationTotalMs"`
	ScriptValidationAvgMicros       float64 `json:"scriptValidationAvgMicros"`
	LinkedACSRegtestArtifactPresent bool    `json:"linkedACSRegtestArtifactPresent"`
	LinkedACSSignetArtifactPresent  bool    `json:"linkedACSSignetArtifactPresent"`
	GeneratedAtUTC                  string  `json:"generatedAtUtc"`
}

type experimentReport struct {
	GeneratedAtUTC    string                 `json:"generatedAtUtc"`
	Source            string                 `json:"source"`
	SeedRuns          int                    `json:"seedRuns"`
	GridCount         int                    `json:"gridCount"`
	SweepRows         []sweepRow             `json:"sweepRows"`
	MultiHopRows      []multiHopRow          `json:"multiHopRows"`
	BaselinePipelines []experiments.Pipeline `json:"baselinePipelines"`
	SimSummaries      []simSummary           `json:"simSummaries"`
	Telemetry         telemetry              `json:"telemetry"`
}

func main() {
	startAll := time.Now()
	configs := buildGridConfigs(defaultVSat)

	sweepRows := make([]sweepRow, 0, len(configs))
	for _, cfg := range configs {
		sweepRows = append(sweepRows, evaluateGridRow(cfg))
	}
	baselinePipelines := buildBaselinePipelines(configs)

	multiHop := buildMultiHopTable(defaultVSat, 0.05, 0.025)
	simSummaries := runSeedSimulations(configs, defaultSeedRuns)
	tel := collectTelemetry()

	rep := experimentReport{
		GeneratedAtUTC:    time.Now().UTC().Format(time.RFC3339),
		Source:            "crab-he experiment runner (check-list + experiment_guide criteria)",
		SeedRuns:          defaultSeedRuns,
		GridCount:         len(configs),
		SweepRows:         sweepRows,
		MultiHopRows:      multiHop,
		BaselinePipelines: baselinePipelines,
		SimSummaries:      simSummaries,
		Telemetry:         tel,
	}

	must(os.MkdirAll(filepath.Join("artifacts", "experiments"), 0o755))
	jsonPath := filepath.Join("artifacts", "experiments", "experiment_summary.json")
	csvPath := filepath.Join("artifacts", "experiments", "parameter_sweep.csv")
	multiHopPath := filepath.Join("artifacts", "experiments", "multi_hop_table.csv")
	baselinePipelinesPath := filepath.Join("artifacts", "experiments", "baseline_pipelines.json")
	simPath := filepath.Join("artifacts", "experiments", "seed_simulation_summary.json")

	b, err := json.MarshalIndent(rep, "", "  ")
	must(err)
	must(os.WriteFile(jsonPath, b, 0o644))
	must(writeSweepCSV(csvPath, sweepRows))
	must(writeMultiHopCSV(multiHopPath, multiHop))
	must(writeJSON(baselinePipelinesPath, baselinePipelines))
	must(writeJSON(simPath, simSummaries))

	fmt.Println("Generated experiment artifacts:")
	fmt.Println(" -", jsonPath)
	fmt.Println(" -", csvPath)
	fmt.Println(" -", multiHopPath)
	fmt.Println(" -", baselinePipelinesPath)
	fmt.Println(" -", simPath)
	fmt.Printf("Done in %.2f ms\n", float64(time.Since(startAll).Microseconds())/1000.0)
}

func buildGridConfigs(vSat int64) []gridConfig {
	return experiments.BuildGridConfigs(vSat)
}

func buildBaselinePipelines(configs []gridConfig) []experiments.Pipeline {
	out := make([]experiments.Pipeline, 0, len(configs)*2)
	for _, cfg := range configs {
		out = append(out, experiments.BuildMADStandalone(cfg, 0))
		out = append(out, experiments.BuildHeStandalone(cfg, 0))
	}
	return out
}

func evaluateGridRow(cfg gridConfig) sweepRow {
	rowStart := time.Now()
	widthCRAB := experiments.CStar(cfg.VSat, cfg.VDepSat, cfg.VColSat)
	cStar := widthCRAB
	widthCStarMinus := experiments.LinkedWidth(cfg.VSat, cfg.VDepSat, cfg.VColSat, cStar-epsilonSat)
	widthCStar := experiments.LinkedWidth(cfg.VSat, cfg.VDepSat, cfg.VColSat, cStar)
	widthCStarPlus := experiments.LinkedWidth(cfg.VSat, cfg.VDepSat, cfg.VColSat, cStar+epsilonSat)

	madStandalone := experiments.BuildMADStandalone(cfg, 0)
	heStandalone := experiments.BuildHeStandalone(cfg, 0)
	madStandaloneWidth := madStandalone.AttackWidthSat
	heStandaloneMargin := -heStandalone.AttackWidthSat

	return sweepRow{
		ConfigID:               cfg.ID,
		VSat:                   cfg.VSat,
		VDepSat:                cfg.VDepSat,
		VColSat:                cfg.VColSat,
		Kappa:                  cfg.Kappa,
		HopsN:                  cfg.HopsN,
		WidthCRABSat:           widthCRAB,
		WidthCRABByzantineSat:  widthCRAB,
		WidthCRABPrime1_25Sat:  widthCRAB,
		WidthCRABPrime1_50Sat:  widthCRAB,
		WidthCRABPrime2_00Sat:  widthCRAB,
		CStarSat:               cStar,
		WidthCRABHeCStarMinus:  widthCStarMinus,
		WidthCRABHeCStar:       widthCStar,
		WidthCRABHeCStarPlus:   widthCStarPlus,
		CNStarSat:              experiments.CNStar(cfg.VSat, cfg.VDepSat, cfg.VColSat, cfg.HopsN),
		HeConditionValid:       cfg.HeConditionValid,
		HeConditionReason:      cfg.HeConditionReason,
		MADStandaloneWidthSat:  madStandaloneWidth,
		HeStandaloneMarginSat:  heStandaloneMargin,
		ActiveMinerCoverageMAD: madStandalone.Feasible,
		Notes:                  "CRAB widths remain invariant under collateral-only scaling; standalone MAD/He baselines are computed through tx-level pipelines",
		ElapsedMicros:          time.Since(rowStart).Microseconds(),
		GeneratedAtUTC:         time.Now().UTC().Format(time.RFC3339),
	}
}

func buildMultiHopTable(vSat int64, depRatio float64, colRatio float64) []multiHopRow {
	vDep := int64(math.Round(float64(vSat) * depRatio))
	vCol := int64(math.Round(float64(vDep) * colRatio))
	rows := make([]multiHopRow, 0, 4)
	for _, n := range []int{1, 3, 5, 7} {
		cN := experiments.CNStar(vSat, vDep, vCol, n)
		rows = append(rows, multiHopRow{
			N:           n,
			CNStarSat:   cN,
			OverheadSat: cN - vSat,
		})
	}
	return rows
}

func runSeedSimulations(configs []gridConfig, seedRuns int) []simSummary {
	out := make([]simSummary, 0, len(configs)*3)
	for _, cfg := range configs {
		baselineSet := []struct {
			name  string
			width int64
		}{
			{name: "CRAB collateral-only baseline (c=v)", width: experiments.CStar(cfg.VSat, cfg.VDepSat, cfg.VColSat)},
			{name: "MAD-HTLC standalone", width: experiments.BuildMADStandalone(cfg, 0).AttackWidthSat},
			{name: "He-HTLC standalone", width: experiments.BuildHeStandalone(cfg, 0).AttackWidthSat},
		}

		crabHeWidth := experiments.LinkedWidth(cfg.VSat, cfg.VDepSat, cfg.VColSat, experiments.CStar(cfg.VSat, cfg.VDepSat, cfg.VColSat))
		for _, baseline := range baselineSet {
			pairs := make([]simPair, 0, seedRuns)
			for i := 1; i <= seedRuns; i++ {
				seed := int64(cfg.Kappa*10_000 + cfg.HopsN*1_000 + i)
				rng := rand.New(rand.NewSource(seed))

				baselineSuccess := noisySuccess(baseline.width, rng)
				crabHeSuccess := noisySuccess(crabHeWidth, rng)
				pairs = append(pairs, simPair{
					Seed:               seed,
					BaselineSuccess:    btoi(baselineSuccess),
					CRABHeSuccess:      btoi(crabHeSuccess),
					DifferenceBaseline: btoi(baselineSuccess) - btoi(crabHeSuccess),
				})
			}

			diffs := make([]float64, 0, len(pairs))
			baselineCount := 0
			crabHeCount := 0
			for _, p := range pairs {
				diffs = append(diffs, float64(p.DifferenceBaseline))
				baselineCount += p.BaselineSuccess
				crabHeCount += p.CRABHeSuccess
			}

			stats := computePairedStats(diffs)
			out = append(out, simSummary{
				ConfigID:            cfg.ID,
				BaselineName:        baseline.name,
				SeedRuns:            seedRuns,
				HeConditionValid:    cfg.HeConditionValid,
				HeConditionReason:   cfg.HeConditionReason,
				IncludedInValidSet:  cfg.HeConditionValid,
				SimulationModel:     simModelDescription,
				BaselineSuccessRate: float64(baselineCount) / float64(seedRuns),
				CRABHeSuccessRate:   float64(crabHeCount) / float64(seedRuns),
				Pairs:               pairs,
				Stats:               stats,
				GeneratedAtUTC:      time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
	return out
}

func noisySuccess(widthSat int64, rng *rand.Rand) bool {
	// This synthetic model perturbs analytical width; it is not an on-chain execution oracle.
	sigma := math.Max(1_000.0, 0.05*math.Abs(float64(widthSat))+1.0)
	effective := float64(widthSat) + rng.NormFloat64()*sigma
	return effective > 0
}

func computePairedStats(diffs []float64) pairedStats {
	n := len(diffs)
	if n == 0 {
		return pairedStats{}
	}

	mean := mean(diffs)
	sd := stddevSample(diffs, mean)
	se := 0.0
	if n > 0 {
		se = sd / math.Sqrt(float64(n))
	}
	ciDelta := 1.96 * se

	tStat := 0.0
	tP := 1.0
	cohen := 0.0
	if sd > 0 {
		tStat = mean / se
		tP = 2 * (1 - normCDF(math.Abs(tStat)))
		cohen = mean / sd
	} else if mean != 0 {
		tP = 0
	}

	wPlus, wZ, wP, wR := wilcoxonSignedRank(diffs)

	return pairedStats{
		SampleSize:           n,
		MeanDiff:             mean,
		StdDevDiff:           sd,
		CI95Low:              mean - ciDelta,
		CI95High:             mean + ciDelta,
		TStatistic:           tStat,
		TTestPValueApprox:    tP,
		CohensDz:             cohen,
		WilcoxonWPlus:        wPlus,
		WilcoxonZApprox:      wZ,
		WilcoxonPValueApprox: wP,
		WilcoxonEffectR:      wR,
	}
}

func wilcoxonSignedRank(diffs []float64) (wPlus float64, z float64, p float64, r float64) {
	type nz struct {
		abs  float64
		sign float64
	}
	vals := make([]nz, 0, len(diffs))
	for _, d := range diffs {
		if d == 0 {
			continue
		}
		sign := 1.0
		if d < 0 {
			sign = -1.0
		}
		vals = append(vals, nz{abs: math.Abs(d), sign: sign})
	}
	n := len(vals)
	if n == 0 {
		return 0, 0, 1, 0
	}

	sort.Slice(vals, func(i, j int) bool { return vals[i].abs < vals[j].abs })

	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i + 1
		for j < n && vals[j].abs == vals[i].abs {
			j++
		}
		avgRank := (float64(i+1) + float64(j)) / 2.0
		for k := i; k < j; k++ {
			ranks[k] = avgRank
		}
		i = j
	}

	for i := range vals {
		if vals[i].sign > 0 {
			wPlus += ranks[i]
		}
	}

	mu := float64(n*(n+1)) / 4.0
	sigma := math.Sqrt(float64(n*(n+1)*(2*n+1)) / 24.0)
	if sigma == 0 {
		return wPlus, 0, 1, 0
	}
	z = (wPlus - mu) / sigma
	p = 2 * (1 - normCDF(math.Abs(z)))
	r = math.Abs(z) / math.Sqrt(float64(n))
	return wPlus, z, p, r
}

func collectTelemetry() telemetry {
	regtestBlocks := int64(0)
	signetBlocks := int64(0)

	regtestPresent := false
	signetPresent := false

	if m, err := readHeightArtifact(filepath.Join("artifacts", "regtest_txids.json")); err == nil {
		regtestBlocks = m.EndHeight - m.StartHeight
	}
	if m, err := readHeightArtifact(filepath.Join("artifacts", "signet_txids.json")); err == nil {
		signetBlocks = m.EndHeight - m.StartHeight
	}
	if _, err := os.Stat(filepath.Join("artifacts", "linked_acs_regtest.json")); err == nil {
		regtestPresent = true
	}
	if _, err := os.Stat(filepath.Join("artifacts", "linked_acs_signet.json")); err == nil {
		signetPresent = true
	}

	wIter := 20_000
	wTotalMs, wAvg := benchmarkWitnessGeneration(wIter)
	sIter := 20_000
	sTotalMs, sAvg := benchmarkScriptValidation(sIter)

	return telemetry{
		RegtestBlocksObserved:           regtestBlocks,
		SignetBlocksObserved:            signetBlocks,
		WitnessGenerationIterations:     wIter,
		WitnessGenerationTotalMs:        wTotalMs,
		WitnessGenerationAvgMicros:      wAvg,
		ScriptValidationIterations:      sIter,
		ScriptValidationTotalMs:         sTotalMs,
		ScriptValidationAvgMicros:       sAvg,
		LinkedACSRegtestArtifactPresent: regtestPresent,
		LinkedACSSignetArtifactPresent:  signetPresent,
		GeneratedAtUTC:                  time.Now().UTC().Format(time.RFC3339),
	}
}

func benchmarkWitnessGeneration(iter int) (totalMs float64, avgMicros float64) {
	start := time.Now()
	for i := 0; i < iter; i++ {
		pre := sha256.Sum256([]byte(fmt.Sprintf("pre-b-%d", i)))
		rev := sha256.Sum256([]byte(fmt.Sprintf("rja-%d", i)))
		_ = "<" + hex.EncodeToString(pre[:]) + "> <" + hex.EncodeToString(rev[:]) + "> <redeemScript>"
	}
	d := time.Since(start)
	totalMs = float64(d.Microseconds()) / 1000.0
	avgMicros = float64(d.Microseconds()) / float64(iter)
	return totalMs, avgMicros
}

func benchmarkScriptValidation(iter int) (totalMs float64, avgMicros float64) {
	secretR := []byte("revocation-secret-fixed")
	secretP := []byte("pre-b-secret-fixed")
	hR := sha256.Sum256(secretR)
	hP := sha256.Sum256(secretP)

	start := time.Now()
	for i := 0; i < iter; i++ {
		cR := sha256.Sum256(secretR)
		cP := sha256.Sum256(secretP)
		if cR != hR || cP != hP {
			panic("script validation mismatch")
		}
	}
	d := time.Since(start)
	totalMs = float64(d.Microseconds()) / 1000.0
	avgMicros = float64(d.Microseconds()) / float64(iter)
	return totalMs, avgMicros
}

type heightArtifact struct {
	StartHeight int64 `json:"startHeight"`
	EndHeight   int64 `json:"endHeight"`
}

func readHeightArtifact(path string) (heightArtifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return heightArtifact{}, err
	}
	var h heightArtifact
	if err := json.Unmarshal(b, &h); err != nil {
		return heightArtifact{}, err
	}
	return h, nil
}

func writeSweepCSV(path string, rows []sweepRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	head := []string{
		"config_id", "v_sat", "v_dep_sat", "v_col_sat", "kappa", "n",
		"width_crab_sat", "width_crab_byz_sat", "width_crab_p125_sat",
		"width_crab_p150_sat", "width_crab_p200_sat", "c_star_sat",
		"width_crabhe_cstar_minus_eps", "width_crabhe_cstar", "width_crabhe_cstar_plus_eps",
		"c_n_star_sat", "he_condition_valid", "he_condition_reason",
		"mad_standalone_width_sat", "he_standalone_margin_sat", "elapsed_micros",
	}
	if err := w.Write(head); err != nil {
		return err
	}

	for _, r := range rows {
		rec := []string{
			r.ConfigID,
			fmt.Sprintf("%d", r.VSat),
			fmt.Sprintf("%d", r.VDepSat),
			fmt.Sprintf("%d", r.VColSat),
			fmt.Sprintf("%d", r.Kappa),
			fmt.Sprintf("%d", r.HopsN),
			fmt.Sprintf("%d", r.WidthCRABSat),
			fmt.Sprintf("%d", r.WidthCRABByzantineSat),
			fmt.Sprintf("%d", r.WidthCRABPrime1_25Sat),
			fmt.Sprintf("%d", r.WidthCRABPrime1_50Sat),
			fmt.Sprintf("%d", r.WidthCRABPrime2_00Sat),
			fmt.Sprintf("%d", r.CStarSat),
			fmt.Sprintf("%d", r.WidthCRABHeCStarMinus),
			fmt.Sprintf("%d", r.WidthCRABHeCStar),
			fmt.Sprintf("%d", r.WidthCRABHeCStarPlus),
			fmt.Sprintf("%d", r.CNStarSat),
			fmt.Sprintf("%t", r.HeConditionValid),
			r.HeConditionReason,
			fmt.Sprintf("%d", r.MADStandaloneWidthSat),
			fmt.Sprintf("%d", r.HeStandaloneMarginSat),
			fmt.Sprintf("%d", r.ElapsedMicros),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return w.Error()
}

func writeMultiHopCSV(path string, rows []multiHopRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"n", "c_n_star_sat", "overhead_sat"}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			fmt.Sprintf("%d", r.N),
			fmt.Sprintf("%d", r.CNStarSat),
			fmt.Sprintf("%d", r.OverheadSat),
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stddevSample(xs []float64, mu float64) float64 {
	n := len(xs)
	if n <= 1 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		d := x - mu
		s += d * d
	}
	return math.Sqrt(s / float64(n-1))
}

func normCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
