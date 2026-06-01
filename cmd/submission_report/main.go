package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type linkedArtifact struct {
	Network      string `json:"network"`
	FundTxid     string `json:"fundTxid"`
	SpendTxid    string `json:"spendTxid"`
	FundValueSat int64  `json:"fundValueSat"`
	BurnValueSat int64  `json:"burnValueSat"`
	FeeSat       int64  `json:"feeSat"`
	MinerPayout  int64  `json:"minerPayoutSat"`
	CreatedAtUTC string `json:"createdAtUtc"`
}

func main() {
	outPath := filepath.Join("artifacts", "submission_report.md")
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	var b strings.Builder
	b.WriteString("# CRAB-He Submission Artifact Report\n\n")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("This report summarizes repository artifacts used by the CRAB-He paper. It is an evidence index, not a new theorem or a production-deployment claim.\n\n")

	writeAttackTimeline(&b)
	writeParallelSwaps(&b)
	writeLinkedArtifacts(&b)
	writeFeeProfiles(&b)
	writeNonClaims(&b)

	must(os.MkdirAll(filepath.Dir(outPath), 0o755))
	must(os.WriteFile(outPath, []byte(b.String()), 0o644))
	fmt.Println("wrote", outPath)
}

func writeAttackTimeline(b *strings.Builder) {
	b.WriteString("## Attack Timeline Replay\n\n")
	rows, err := readCSV(filepath.Join("artifacts", "experiments", "attack_timeline.csv"))
	if err != nil {
		b.WriteString(fmt.Sprintf("- Missing or unreadable attack timeline: `%v`\n\n", err))
		return
	}
	writeMarkdownTable(b, rows)
	b.WriteString("\n")
}

func writeParallelSwaps(b *strings.Builder) {
	b.WriteString("## Parallel Independent Swaps\n\n")
	rows, err := readCSV(filepath.Join("artifacts", "experiments", "parallel_swaps_table.csv"))
	if err != nil {
		b.WriteString(fmt.Sprintf("- Missing or unreadable parallel-swap table: `%v`\n\n", err))
		return
	}
	b.WriteString("Invariant: `c*_n = v + n*v_dep` for independent standalone HTLC instances; this is not a routed-Lightning multi-hop claim.\n\n")
	writeMarkdownTable(b, rows)
	b.WriteString("\n")
}

func writeLinkedArtifacts(b *strings.Builder) {
	b.WriteString("## Linked ACS Evidence\n\n")
	rows := [][]string{{"network", "fundTxid", "spendTxid", "fundSat", "burnSat", "feeSat", "createdAtUtc"}}
	for _, network := range []string{"regtest", "signet"} {
		path := filepath.Join("artifacts", fmt.Sprintf("linked_acs_%s.json", network))
		var art linkedArtifact
		if err := readJSON(path, &art); err != nil {
			rows = append(rows, []string{network, "missing", "missing", "-", "-", "-", "-"})
			continue
		}
		rows = append(rows, []string{
			art.Network,
			short(art.FundTxid),
			short(art.SpendTxid),
			fmt.Sprintf("%d", art.FundValueSat),
			fmt.Sprintf("%d", art.BurnValueSat),
			fmt.Sprintf("%d", art.FeeSat),
			art.CreatedAtUTC,
		})
	}
	writeMarkdownTable(b, rows)
	b.WriteString("\n")
}

func writeFeeProfiles(b *strings.Builder) {
	b.WriteString("## Fee-Profile Campaign\n\n")
	rows := [][]string{{"network", "runs", "successful"}}
	for _, network := range []string{"regtest", "signet"} {
		path := filepath.Join("artifacts", "onchain", network, "fee_profiles", "fee_profile_summary.csv")
		records, err := readCSV(path)
		if err != nil || len(records) < 2 {
			rows = append(rows, []string{network, "missing", "missing"})
			continue
		}
		success := 0
		for _, row := range records[1:] {
			if len(row) >= 3 && strings.EqualFold(row[2], "true") {
				success++
			}
		}
		rows = append(rows, []string{network, fmt.Sprintf("%d", len(records)-1), fmt.Sprintf("%d", success)})
	}
	writeMarkdownTable(b, rows)
	b.WriteString("\n")
}

func writeNonClaims(b *strings.Builder) {
	b.WriteString("## Non-Claims\n\n")
	b.WriteString("- No production miner bribery marketplace.\n")
	b.WriteString("- No modified Bitcoin miner client performing real censorship.\n")
	b.WriteString("- No confirmed CLBA mainnet incident.\n")
	b.WriteString("- No full routed-Lightning HTLC model.\n")
	b.WriteString("- No theorem-level multi-miner coalition game.\n")
}

func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return csv.NewReader(f).ReadAll()
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func writeMarkdownTable(b *strings.Builder, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	writeRow := func(row []string) {
		b.WriteString("| ")
		for i, cell := range row {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(escapeCell(cell))
		}
		b.WriteString(" |\n")
	}
	writeRow(rows[0])
	b.WriteString("|")
	for range rows[0] {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, row := range rows[1:] {
		writeRow(row)
	}
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func short(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:8] + "..." + s[len(s)-8:]
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
