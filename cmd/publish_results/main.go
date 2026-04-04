package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type sweepRow struct {
	ConfigID              string `json:"configId"`
	VDepSat               int64  `json:"vDepSat"`
	VColSat               int64  `json:"vColSat"`
	Kappa                 int    `json:"kappa"`
	HopsN                 int    `json:"hopsN"`
	WidthCRABSat          int64  `json:"widthCRABSat"`
	CStarSat              int64  `json:"cStarSat"`
	WidthCRABHeCStar      int64  `json:"widthCRABHeCStarSat"`
	MADStandaloneWidthSat int64  `json:"madStandaloneWidthSat"`
	HeStandaloneMarginSat int64  `json:"heStandaloneMarginSat"`
}

type multiHopRow struct {
	N           int   `json:"n"`
	CNStarSat   int64 `json:"cNStarSat"`
	OverheadSat int64 `json:"overheadAboveCRABByzSat"`
}

type experimentSummary struct {
	SweepRows    []sweepRow    `json:"sweepRows"`
	MultiHopRows []multiHopRow `json:"multiHopRows"`
}

type simSummary struct {
	ConfigID            string  `json:"configId"`
	BaselineName        string  `json:"baselineName"`
	BaselineSuccessRate float64 `json:"baselineSuccessRate"`
	CRABHeSuccessRate   float64 `json:"crabHeSuccessRate"`
	Stats               struct {
		MeanDiff             float64 `json:"meanDiff"`
		TTestPValueApprox    float64 `json:"tTestPValueApprox"`
		WilcoxonPValueApprox float64 `json:"wilcoxonPValueApprox"`
	} `json:"stats"`
}

type onchainSummary struct {
	TotalRuns        int `json:"totalRuns"`
	SuccessfulRuns   int `json:"successfulRuns"`
	FailedRuns       int `json:"failedRuns"`
	NetworkSummaries []struct {
		Network     string  `json:"network"`
		TotalRuns   int     `json:"totalRuns"`
		SuccessRuns int     `json:"successRuns"`
		SuccessRate float64 `json:"successRate"`
	} `json:"networkSummaries"`
}

func main() {
	var (
		experimentPath = flag.String("experiment-summary", filepath.Join("artifacts", "experiments", "experiment_summary.json"), "Path to experiment summary JSON")
		simPath        = flag.String("seed-summary", filepath.Join("artifacts", "experiments", "seed_simulation_summary.json"), "Path to seed simulation summary JSON")
		onchainPath    = flag.String("onchain-summary", filepath.Join("artifacts", "onchain", "repeated_onchain_summary.json"), "Path to repeated on-chain summary JSON")
		outDir         = flag.String("out", filepath.Join("artifacts", "publication"), "Output directory for publication assets")
	)
	flag.Parse()

	var exp experimentSummary
	must(readJSON(*experimentPath, &exp))

	var sims []simSummary
	must(readJSON(*simPath, &sims))

	onchain := onchainSummary{}
	hasOnchain := false
	if _, err := os.Stat(*onchainPath); err == nil {
		must(readJSON(*onchainPath, &onchain))
		hasOnchain = true
	}

	must(os.MkdirAll(*outDir, 0o755))

	rows := selectRepresentativeRows(exp.SweepRows)
	must(os.WriteFile(filepath.Join(*outDir, "table_main_results.tex"), []byte(buildMainResultsTex(rows)), 0o644))
	must(os.WriteFile(filepath.Join(*outDir, "table_multi_hop.tex"), []byte(buildMultiHopTex(exp.MultiHopRows)), 0o644))
	must(os.WriteFile(filepath.Join(*outDir, "table_seed_stats.tex"), []byte(buildSeedStatsTex(sims)), 0o644))
	if hasOnchain {
		must(os.WriteFile(filepath.Join(*outDir, "table_onchain_runs.tex"), []byte(buildOnchainTex(onchain)), 0o644))
	}

	must(os.WriteFile(filepath.Join(*outDir, "fig_multi_hop_cnstar.svg"), []byte(drawMultiHopSVG(exp.MultiHopRows)), 0o644))
	must(os.WriteFile(filepath.Join(*outDir, "fig_baseline_success.svg"), []byte(drawBaselineSuccessSVG(sims)), 0o644))
	if hasOnchain {
		must(os.WriteFile(filepath.Join(*outDir, "fig_onchain_success.svg"), []byte(drawOnchainSVG(onchain)), 0o644))
	}

	manifest := map[string]any{
		"generatedAtUtc": time.Now().UTC().Format(time.RFC3339),
		"sources": map[string]string{
			"experimentSummary": *experimentPath,
			"seedSummary":       *simPath,
			"onchainSummary":    *onchainPath,
		},
	}
	must(writeJSON(filepath.Join(*outDir, "publication_manifest.json"), manifest))

	fmt.Println("Generated publication assets in", *outDir)
}

func selectRepresentativeRows(rows []sweepRow) []sweepRow {
	selected := make([]sweepRow, 0, 4)

	find := func(match func(sweepRow) bool) (sweepRow, bool) {
		for _, r := range rows {
			if match(r) {
				return r, true
			}
		}
		return sweepRow{}, false
	}

	if r, ok := find(func(r sweepRow) bool { return r.VDepSat == r.VColSat && r.Kappa == 3 && r.HopsN == 1 }); ok {
		selected = append(selected, r)
	}
	if r, ok := find(func(r sweepRow) bool {
		return r.VDepSat == 100000 && r.VColSat == 50000 && r.Kappa == 3 && r.HopsN == 1
	}); ok {
		selected = append(selected, r)
	}
	if r, ok := find(func(r sweepRow) bool {
		return r.VDepSat == 100000 && r.VColSat == 50000 && r.Kappa == 3 && r.HopsN == 5
	}); ok {
		selected = append(selected, r)
	}
	if r, ok := find(func(r sweepRow) bool {
		return r.VDepSat == 100000 && r.VColSat == 50000 && r.Kappa == 3 && r.HopsN == 7
	}); ok {
		selected = append(selected, r)
	}

	if len(selected) == 0 {
		copyRows := make([]sweepRow, len(rows))
		copy(copyRows, rows)
		sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].ConfigID < copyRows[j].ConfigID })
		if len(copyRows) > 4 {
			copyRows = copyRows[:4]
		}
		return copyRows
	}
	return selected
}

func buildMainResultsTex(rows []sweepRow) string {
	b := &strings.Builder{}
	b.WriteString("\\begin{table}[t]\n")
	b.WriteString("\\caption{Representative parameter outcomes for publication.}\n")
	b.WriteString("\\label{tab:pub_main_results}\n")
	b.WriteString("\\centering\n")
	b.WriteString("\\small\n")
	b.WriteString("\\setlength{\\tabcolsep}{3pt}\n")
	b.WriteString("\\resizebox{\\columnwidth}{!}{%\n")
	b.WriteString("\\begin{tabular}{lrrrrrr}\n")
	b.WriteString("\\toprule\n")
	b.WriteString("Scenario & n & c$^*$ (sat) & W$_{CRAB}$ & W$_{CRAB-He}$ & W$_{MAD}$ & Margin$_{He}$ \\\\ \n")
	b.WriteString("\\midrule\n")

	for _, r := range rows {
		name := scenarioLabel(r)
		b.WriteString(fmt.Sprintf("%s & %d & %d & %d & %d & %d & %d \\\\ \n",
			latexEscape(name), r.HopsN, r.CStarSat, r.WidthCRABSat, r.WidthCRABHeCStar, r.MADStandaloneWidthSat, r.HeStandaloneMarginSat))
	}

	b.WriteString("\\bottomrule\n")
	b.WriteString("\\end{tabular}%\n")
	b.WriteString("}\n")
	b.WriteString("\\end{table}\n")
	return b.String()
}

func buildMultiHopTex(rows []multiHopRow) string {
	copyRows := make([]multiHopRow, len(rows))
	copy(copyRows, rows)
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].N < copyRows[j].N })

	b := &strings.Builder{}
	b.WriteString("\\begin{table}[t]\n")
	b.WriteString("\\caption{Multi-hop collateral threshold from generated artifacts.}\n")
	b.WriteString("\\label{tab:pub_multi_hop}\n")
	b.WriteString("\\centering\n")
	b.WriteString("\\begin{tabular}{rrr}\n")
	b.WriteString("\\toprule\n")
	b.WriteString("n & c$_n^*$ (sat) & overhead (sat) \\\\ \n")
	b.WriteString("\\midrule\n")
	for _, r := range copyRows {
		b.WriteString(fmt.Sprintf("%d & %d & %d \\\\ \n", r.N, r.CNStarSat, r.OverheadSat))
	}
	b.WriteString("\\bottomrule\n")
	b.WriteString("\\end{tabular}\n")
	b.WriteString("\\end{table}\n")
	return b.String()
}

func buildSeedStatsTex(sims []simSummary) string {
	filtered := make([]simSummary, 0)
	for _, s := range sims {
		if strings.Contains(s.ConfigID, "dep0.050-col0.50") && strings.Contains(s.ConfigID, "-k3-") {
			if strings.HasSuffix(s.ConfigID, "-n1") || strings.HasSuffix(s.ConfigID, "-n5") || strings.HasSuffix(s.ConfigID, "-n7") {
				filtered = append(filtered, s)
			}
		}
	}
	if len(filtered) == 0 {
		filtered = append(filtered, sims...)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].ConfigID == filtered[j].ConfigID {
			return filtered[i].BaselineName < filtered[j].BaselineName
		}
		return filtered[i].ConfigID < filtered[j].ConfigID
	})

	if len(filtered) > 12 {
		filtered = filtered[:12]
	}

	b := &strings.Builder{}
	b.WriteString("\\begin{table}[t]\n")
	b.WriteString("\\caption{Seed-based paired statistics (artifact-selected rows).}\n")
	b.WriteString("\\label{tab:pub_seed_stats}\n")
	b.WriteString("\\centering\n")
	b.WriteString("\\scriptsize\n")
	b.WriteString("\\setlength{\\tabcolsep}{2pt}\n")
	b.WriteString("\\begin{tabularx}{\\columnwidth}{>{\\raggedright\\arraybackslash}X>{\\raggedright\\arraybackslash}Xrrrr}\n")
	b.WriteString("\\toprule\n")
	b.WriteString("Config & Baseline & Rate$_B$ & Rate$_{CRAB-He}$ & Mean diff & p-val (t/W) \\\\ \n")
	b.WriteString("\\midrule\n")
	for _, s := range filtered {
		pPair := fmt.Sprintf("%.1e/%.1e", s.Stats.TTestPValueApprox, s.Stats.WilcoxonPValueApprox)
		b.WriteString(fmt.Sprintf("%s & %s & %.3f & %.3f & %.3f & %s \\\\ \n",
			latexEscape(s.ConfigID), latexEscape(shortBaseline(s.BaselineName)), s.BaselineSuccessRate, s.CRABHeSuccessRate, s.Stats.MeanDiff, pPair))
	}
	b.WriteString("\\bottomrule\n")
	b.WriteString("\\end{tabularx}\n")
	b.WriteString("\\end{table}\n")
	return b.String()
}

func buildOnchainTex(s onchainSummary) string {
	b := &strings.Builder{}
	b.WriteString("\\begin{table}[t]\n")
	b.WriteString("\\caption{Repeated on-chain orchestrator summary.}\n")
	b.WriteString("\\label{tab:pub_onchain}\n")
	b.WriteString("\\centering\n")
	b.WriteString("\\begin{tabular}{lrrr}\n")
	b.WriteString("\\toprule\n")
	b.WriteString("Network & Runs & Success & Success rate \\\\ \n")
	b.WriteString("\\midrule\n")
	for _, n := range s.NetworkSummaries {
		b.WriteString(fmt.Sprintf("%s & %d & %d & %.3f \\\\ \n", latexEscape(n.Network), n.TotalRuns, n.SuccessRuns, n.SuccessRate))
	}
	b.WriteString("\\midrule\n")
	b.WriteString(fmt.Sprintf("Total & %d & %d & %.3f \\\\ \n", s.TotalRuns, s.SuccessfulRuns, safeRate(s.SuccessfulRuns, s.TotalRuns)))
	b.WriteString("\\bottomrule\n")
	b.WriteString("\\end{tabular}\n")
	b.WriteString("\\end{table}\n")
	return b.String()
}

func drawMultiHopSVG(rows []multiHopRow) string {
	if len(rows) == 0 {
		return emptySVG("No multi-hop data")
	}
	copyRows := make([]multiHopRow, len(rows))
	copy(copyRows, rows)
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].N < copyRows[j].N })

	xMin, xMax := float64(copyRows[0].N), float64(copyRows[len(copyRows)-1].N)
	yMin, yMax := float64(copyRows[0].CNStarSat), float64(copyRows[0].CNStarSat)
	for _, r := range copyRows {
		v := float64(r.CNStarSat)
		if v < yMin {
			yMin = v
		}
		if v > yMax {
			yMax = v
		}
	}
	if yMax == yMin {
		yMax = yMin + 1
	}

	width, height := 960.0, 560.0
	l, r, t, btm := 100.0, 60.0, 60.0, 90.0
	pw, ph := width-l-r, height-t-btm

	mapX := func(v float64) float64 {
		if xMax == xMin {
			return l + pw/2
		}
		return l + (v-xMin)/(xMax-xMin)*pw
	}
	mapY := func(v float64) float64 {
		return t + (1-(v-yMin)/(yMax-yMin))*ph
	}

	linePts := make([]string, 0, len(copyRows))
	dots := &strings.Builder{}
	for _, row := range copyRows {
		x := mapX(float64(row.N))
		y := mapY(float64(row.CNStarSat))
		linePts = append(linePts, fmt.Sprintf("%.1f,%.1f", x, y))
		dots.WriteString(fmt.Sprintf("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"5\" fill=\"#1f77b4\"/>", x, y))
		dots.WriteString(fmt.Sprintf("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" text-anchor=\"middle\" fill=\"#222\">n=%d</text>", x, y-10, row.N))
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">
<rect width="100%%" height="100%%" fill="#ffffff"/>
<text x="%.0f" y="32" font-size="26" font-family="Times New Roman" fill="#111">Multi-hop Collateral Threshold</text>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
<polyline fill="none" stroke="#1f77b4" stroke-width="3" points="%s"/>
%s
<text x="%.1f" y="%.1f" font-size="18" text-anchor="middle" font-family="Times New Roman">Hop count n</text>
<text transform="translate(28,%.1f) rotate(-90)" font-size="18" text-anchor="middle" font-family="Times New Roman">c_n^* (sat)</text>
</svg>`,
		width, height, width, height,
		width/2,
		l, t+ph, l+pw, t+ph,
		l, t, l, t+ph,
		strings.Join(linePts, " "),
		dots.String(),
		l+pw/2, height-28,
		t+ph/2)
}

func drawBaselineSuccessSVG(sims []simSummary) string {
	if len(sims) == 0 {
		return emptySVG("No seed summary data")
	}
	baselineOrder := []string{"CRAB collateral-only baseline (c=v)", "MAD-HTLC standalone", "He-HTLC standalone"}
	vals := make(map[string]float64)
	for _, b := range baselineOrder {
		vals[b] = 0
	}
	counts := make(map[string]int)

	for _, s := range sims {
		if _, ok := vals[s.BaselineName]; ok {
			vals[s.BaselineName] += s.BaselineSuccessRate
			counts[s.BaselineName]++
		}
	}
	for _, b := range baselineOrder {
		if counts[b] > 0 {
			vals[b] /= float64(counts[b])
		}
	}

	width, height := 960.0, 560.0
	l, r, t, btm := 120.0, 80.0, 60.0, 120.0
	pw, ph := width-l-r, height-t-btm

	barW := pw / float64(len(baselineOrder)) * 0.55
	gap := pw / float64(len(baselineOrder))

	colors := []string{"#2a9d8f", "#e76f51", "#457b9d"}
	bars := &strings.Builder{}
	labels := &strings.Builder{}
	for i, bName := range baselineOrder {
		x := l + gap*float64(i) + (gap-barW)/2
		h := vals[bName] * ph
		y := t + (ph - h)
		bars.WriteString(fmt.Sprintf("<rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"%s\"/>", x, y, barW, h, colors[i]))
		bars.WriteString(fmt.Sprintf("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" text-anchor=\"middle\" fill=\"#111\">%.3f</text>", x+barW/2, y-8, vals[bName]))
		labels.WriteString(fmt.Sprintf("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" text-anchor=\"middle\" fill=\"#222\" transform=\"rotate(20 %.1f %.1f)\">%s</text>",
			x+barW/2, height-56, x+barW/2, height-56, svgEscape(shortBaseline(bName))))
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">
<rect width="100%%" height="100%%" fill="#ffffff"/>
<text x="%.0f" y="32" font-size="26" font-family="Times New Roman" fill="#111">Average Baseline Attack Success Rate</text>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
%s
%s
<text x="%.1f" y="%.1f" font-size="18" text-anchor="middle" font-family="Times New Roman">Baseline</text>
<text transform="translate(34,%.1f) rotate(-90)" font-size="18" text-anchor="middle" font-family="Times New Roman">Success rate</text>
</svg>`,
		width, height, width, height,
		width/2,
		l, t+ph, l+pw, t+ph,
		l, t, l, t+ph,
		bars.String(),
		labels.String(),
		l+pw/2, height-18,
		t+ph/2)
}

func drawOnchainSVG(summary onchainSummary) string {
	if len(summary.NetworkSummaries) == 0 {
		return emptySVG("No on-chain summary data")
	}
	width, height := 780.0, 460.0
	l, r, t, btm := 90.0, 40.0, 60.0, 100.0
	pw, ph := width-l-r, height-t-btm
	barW := pw / float64(len(summary.NetworkSummaries)) * 0.55
	gap := pw / float64(len(summary.NetworkSummaries))

	bars := &strings.Builder{}
	for i, n := range summary.NetworkSummaries {
		x := l + gap*float64(i) + (gap-barW)/2
		h := n.SuccessRate * ph
		y := t + (ph - h)
		bars.WriteString(fmt.Sprintf("<rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"#264653\"/>", x, y, barW, h))
		bars.WriteString(fmt.Sprintf("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" text-anchor=\"middle\">%.3f</text>", x+barW/2, y-8, n.SuccessRate))
		bars.WriteString(fmt.Sprintf("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" text-anchor=\"middle\">%s</text>", x+barW/2, height-50, svgEscape(n.Network)))
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">
<rect width="100%%" height="100%%" fill="#ffffff"/>
<text x="%.0f" y="32" font-size="24" font-family="Times New Roman">Repeated On-chain Success Rate</text>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
%s
</svg>`,
		width, height, width, height,
		width/2,
		l, t+ph, l+pw, t+ph,
		l, t, l, t+ph,
		bars.String())
}

func scenarioLabel(r sweepRow) string {
	if r.VDepSat == r.VColSat {
		return "Boundary (v_col=v_dep)"
	}
	if r.VDepSat == 100000 && r.VColSat == 50000 {
		switch r.HopsN {
		case 1:
			return "Typical one-hop"
		case 5:
			return "Typical five-hop"
		case 7:
			return "Typical seven-hop"
		}
	}
	return r.ConfigID
}

func shortBaseline(name string) string {
	switch name {
	case "CRAB collateral-only baseline (c=v)":
		return "CRAB collateral-only"
	case "MAD-HTLC standalone":
		return "MAD-HTLC"
	case "He-HTLC standalone":
		return "He-HTLC"
	default:
		return name
	}
}

func safeRate(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func latexEscape(s string) string {
	repl := strings.NewReplacer(
		"_", "\\_",
		"%", "\\%",
		"&", "\\&",
	)
	return repl.Replace(s)
}

func svgEscape(s string) string {
	repl := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return repl.Replace(s)
}

func emptySVG(msg string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="100%%" height="100%%" fill="#fff"/><text x="320" y="180" text-anchor="middle" font-size="20">%s</text></svg>`, svgEscape(msg))
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
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
