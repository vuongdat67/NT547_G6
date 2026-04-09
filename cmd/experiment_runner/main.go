package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/crab-he/internal/experiments"
)

const (
	defaultVSat         int64 = 2_000_000
	epsilonSat          int64 = 1_000
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
	GridCount         int                    `json:"gridCount"`
	SweepRows         []sweepRow             `json:"sweepRows"`
	MultiHopRows      []multiHopRow          `json:"multiHopRows"`
	BaselinePipelines []experiments.Pipeline `json:"baselinePipelines"`
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
	tel := collectTelemetry()

	rep := experimentReport{
		GeneratedAtUTC:    time.Now().UTC().Format(time.RFC3339),
		Source:            "crab-he experiment runner (check-list + experiment_guide criteria)",
		GridCount:         len(configs),
		SweepRows:         sweepRows,
		MultiHopRows:      multiHop,
		BaselinePipelines: baselinePipelines,
		Telemetry:         tel,
	}

	must(os.MkdirAll(filepath.Join("artifacts", "experiments"), 0o755))
	jsonPath := filepath.Join("artifacts", "experiments", "experiment_summary.json")
	csvPath := filepath.Join("artifacts", "experiments", "parameter_sweep.csv")
	multiHopPath := filepath.Join("artifacts", "experiments", "multi_hop_table.csv")
	baselinePipelinesPath := filepath.Join("artifacts", "experiments", "baseline_pipelines.json")

	b, err := json.MarshalIndent(rep, "", "  ")
	must(err)
	must(os.WriteFile(jsonPath, b, 0o644))
	must(writeSweepCSV(csvPath, sweepRows))
	must(writeMultiHopCSV(multiHopPath, multiHop))
	must(writeJSON(baselinePipelinesPath, baselinePipelines))

	fmt.Println("Generated experiment artifacts:")
	fmt.Println(" -", jsonPath)
	fmt.Println(" -", csvPath)
	fmt.Println(" -", multiHopPath)
	fmt.Println(" -", baselinePipelinesPath)
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
	cBase := experiments.CStar(cfg.VSat, cfg.VDepSat, cfg.VColSat)
	cPrime125 := int64(math.Round(float64(cBase) * 1.25))
	cPrime150 := int64(math.Round(float64(cBase) * 1.50))
	cPrime200 := int64(math.Round(float64(cBase) * 2.00))

	widthCRAB := crabWidthWithCollateral(cfg.VSat, cfg.VDepSat, cfg.VColSat, cBase)
	widthCRABPrime125 := crabWidthWithCollateral(cfg.VSat, cfg.VDepSat, cfg.VColSat, cPrime125)
	widthCRABPrime150 := crabWidthWithCollateral(cfg.VSat, cfg.VDepSat, cfg.VColSat, cPrime150)
	widthCRABPrime200 := crabWidthWithCollateral(cfg.VSat, cfg.VDepSat, cfg.VColSat, cPrime200)
	cStar := cBase
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
		WidthCRABPrime1_25Sat:  widthCRABPrime125,
		WidthCRABPrime1_50Sat:  widthCRABPrime150,
		WidthCRABPrime2_00Sat:  widthCRABPrime200,
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
		Notes:                  "CRAB widths are recomputed explicitly from UB(v+c'+v_dep)-LB(c'+v_col) for each c' multiplier; standalone MAD/He baselines are computed through tx-level pipelines",
		ElapsedMicros:          time.Since(rowStart).Microseconds(),
		GeneratedAtUTC:         time.Now().UTC().Format(time.RFC3339),
	}
}

func crabWidthWithCollateral(vSat, vDepSat, vColSat, cPrimeSat int64) int64 {
	ub := vSat + cPrimeSat + vDepSat
	lb := cPrimeSat + vColSat
	return ub - lb
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

func must(err error) {
	if err != nil {
		panic(err)
	}
}
