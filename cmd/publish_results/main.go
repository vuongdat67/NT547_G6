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

type multiHopRow struct {
	N           int   `json:"n"`
	CNStarSat   int64 `json:"cNStarSat"`
	OverheadSat int64 `json:"overheadAboveCRABByzSat"`
}

type experimentSummary struct {
	MultiHopRows []multiHopRow `json:"multiHopRows"`
}

func main() {
	var (
		experimentPath = flag.String("experiment-summary", filepath.Join("artifacts", "experiments", "experiment_summary.json"), "Path to experiment summary JSON")
		outDir         = flag.String("out", filepath.Join("artifacts", "publication"), "Output directory for publication assets")
	)
	flag.Parse()

	var exp experimentSummary
	must(readJSON(*experimentPath, &exp))

	must(os.MkdirAll(*outDir, 0o755))
	removeObsoleteOutputs(*outDir)

	table := []byte(buildMultiHopTex(exp.MultiHopRows))
	must(os.WriteFile(filepath.Join(*outDir, "table_parallel_swaps.tex"), table, 0o644))
	must(os.WriteFile(filepath.Join(*outDir, "table_multi_hop.tex"), table, 0o644))

	fig := []byte(drawMultiHopSVG(exp.MultiHopRows))
	must(os.WriteFile(filepath.Join(*outDir, "fig_parallel_swaps_cnstar.svg"), fig, 0o644))
	must(os.WriteFile(filepath.Join(*outDir, "fig_multi_hop_cnstar.svg"), fig, 0o644))

	manifest := map[string]any{
		"generatedAtUtc": time.Now().UTC().Format(time.RFC3339),
		"sources": map[string]string{
			"experimentSummary": *experimentPath,
		},
	}
	must(writeJSON(filepath.Join(*outDir, "publication_manifest.json"), manifest))

	fmt.Println("Generated publication assets in", *outDir)
}

func removeObsoleteOutputs(outDir string) {
	// Cleanup is intentionally scoped to known legacy publication artifacts so
	// repeated runs do not leave stale files with outdated schemas.
	obsolete := []string{
		"table_main_results.tex",
		"table_seed_stats.tex",
		"table_onchain_runs.tex",
		"fig_baseline_success.svg",
		"fig_baseline_success.pdf",
		"fig_onchain_success.svg",
		"fig_onchain_success.pdf",
	}

	for _, name := range obsolete {
		path := filepath.Join(outDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			must(err)
		}
	}
}

func buildMultiHopTex(rows []multiHopRow) string {
	copyRows := make([]multiHopRow, len(rows))
	copy(copyRows, rows)
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].N < copyRows[j].N })

	b := &strings.Builder{}
	b.WriteString("\\begin{table}[t]\n")
	b.WriteString("\\caption{Collateral threshold for $n$ independent parallel swaps from generated artifacts.}\n")
	b.WriteString("\\label{tab:pub_parallel_swaps}\n")
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

func drawMultiHopSVG(rows []multiHopRow) string {
	if len(rows) == 0 {
		return emptySVG("No parallel-swap data")
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
<text x="%.0f" y="32" font-size="26" font-family="Times New Roman" fill="#111">Parallel-Swap Collateral Threshold</text>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333" stroke-width="2"/>
<polyline fill="none" stroke="#1f77b4" stroke-width="3" points="%s"/>
%s
<text x="%.1f" y="%.1f" font-size="18" text-anchor="middle" font-family="Times New Roman">Number of independent swaps n</text>
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
