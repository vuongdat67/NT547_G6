package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crab-he/internal/channel"
)

type txRow struct {
	Name                string             `json:"name"`
	ObjectType          string             `json:"objectType"`
	VBytes              int                `json:"vbytes"`
	FeeUSDBySatPerVB    map[string]float64 `json:"feeUsdBySatPerVB"`
}

type linkedArtifact struct {
	Network          string `json:"network"`
	Wallet           string `json:"wallet"`
	LinkedACSAddress string `json:"linkedACSAddress"`
	FundTxID         string `json:"fundTxid"`
	FundVout         uint32 `json:"fundVout"`
	FundValueSat     int64  `json:"fundValueSat"`
	SpendTxID        string `json:"spendTxid"`
	SpendValueSat    int64  `json:"spendValueSat"`
	FeeSat           int64  `json:"feeSat"`
	WitnessOrder     string `json:"witnessOrder"`
	CreatedAtUTC     string `json:"createdAtUtc"`
}

type clbaSummary struct {
	CRABRationalWidthSat  string `json:"crabRationalWidthSat"`
	CRABByzantineWidthSat string `json:"crabByzantineWidthSat"`
	CRABHeWidthSat        string `json:"crabHeWidthSat"`
	CRABHeInfeasible      bool   `json:"crabHeInfeasible"`
	CStarSat              string `json:"cStarSat"`
}

type report struct {
	GeneratedAtUTC    string           `json:"generatedAtUtc"`
	Source            string           `json:"source"`
	TxTable           []txRow          `json:"txTable"`
	LinkedDeployments []linkedArtifact `json:"linkedDeployments"`
	CLBASummary       clbaSummary      `json:"clbaSummary"`
}

func main() {
	txRows, clba, err := computeCRABHeData()
	must(err)

	artifacts := []linkedArtifact{}
	if a, err := readLinkedArtifact(filepath.Join("artifacts", "linked_acs_regtest.json")); err == nil {
		artifacts = append(artifacts, a)
	}
	if a, err := readLinkedArtifact(filepath.Join("artifacts", "linked_acs_signet.json")); err == nil {
		artifacts = append(artifacts, a)
	}

	rep := report{
		GeneratedAtUTC:    time.Now().UTC().Format(time.RFC3339),
		Source:            "crab-he local implementation and deployment artifacts",
		TxTable:           txRows,
		LinkedDeployments: artifacts,
		CLBASummary:       clba,
	}

	must(os.MkdirAll("artifacts", 0o755))
	jsonPath := filepath.Join("artifacts", "crab_he_results.json")
	mdPath := filepath.Join("artifacts", "crab_he_results.md")

	b, err := json.MarshalIndent(rep, "", "  ")
	must(err)
	must(os.WriteFile(jsonPath, b, 0o644))
	must(os.WriteFile(mdPath, []byte(toMarkdown(rep)), 0o644))

	fmt.Println("Generated:")
	fmt.Println(" -", jsonPath)
	fmt.Println(" -", mdPath)
}

func computeCRABHeData() ([]txRow, clbaSummary, error) {
	v := big.NewInt(2_000_000)
	vDep := big.NewInt(1_000_000)
	vCol := big.NewInt(500_000)
	delta := big.NewInt(1_000)

	params, err := channel.NewParams(v, vDep, vCol, delta, 144, 288, 6, 3)
	if err != nil {
		return nil, clbaSummary{}, err
	}

	rev := channel.NewRevocationSecret([]byte("0123456789abcdef0123456789abcdef"), 0)
	h := channel.NewHTLCSecrets([]byte("prea-prea-prea-prea-prea-prea-0000"), []byte("preb-preb-preb-preb-preb-preb-1111"))

	commitNoHTLC := channel.MakeCommitA(params, 0, sat(v), 0, rev, nil, "alice_pk_sample", "bob_pk_sample")
	commitHTLC := channel.MakeCommitA(params, 0, sat(v), 0, rev, h, "alice_pk_sample", "bob_pk_sample")

	rows := []txRow{
		newTxRow("tx_fund", "channel_tx", 338),
		newTxRow("tx_commit_A (no HTLC)", "channel_tx", commitNoHTLC.SizeVB),
		newTxRow("tx_commit_A (HTLC+linked ACS)", "channel_tx", commitHTLC.SizeVB),
		newTxRow("tx_spend_A", "channel_tx", 418),
		newTxRow("tx_revoke_B", "channel_tx", 192),
		newTxRow("tx_revoke_ACS_std", "channel_tx", 192),
		newTxRow("tx_revoke_ACS_linked", "channel_tx", 224),
		newTxRow("tx_dep_A", "htlc_tx", 190),
		newTxRow("tx_dep_B", "htlc_tx", 172),
		newTxRow("tx_col_B", "htlc_tx", 152),
		newTxRow("tx_col_M", "htlc_tx", 168),
	}

	attackAnalysis, err := channel.NewCLBAAnalysis(params, 0.3, new(big.Int).Div(v, big.NewInt(2)))
	if err != nil {
		return nil, clbaSummary{}, err
	}
	byzAnalysis, err := channel.NewCLBAAnalysis(params, 0.3, new(big.Int).Set(v))
	if err != nil {
		return nil, clbaSummary{}, err
	}
	defenseAnalysis, err := channel.NewCLBAAnalysis(params, 0.3, params.CStar)
	if err != nil {
		return nil, clbaSummary{}, err
	}

	sum := clbaSummary{
		CRABRationalWidthSat:  attackAnalysis.Width().String(),
		CRABByzantineWidthSat: byzAnalysis.Width().String(),
		CRABHeWidthSat:        defenseAnalysis.WidthLinked().String(),
		CRABHeInfeasible:      !defenseAnalysis.IsCLBAProfitableLinked(),
		CStarSat:              params.CStar.String(),
	}

	return rows, sum, nil
}

func newTxRow(name, objectType string, vb int) txRow {
	return txRow{
		Name:       name,
		ObjectType: objectType,
		VBytes:     vb,
		FeeUSDBySatPerVB: map[string]float64{
			"2":  usdFee(vb, 2),
			"7":  usdFee(vb, 7),
			"10": usdFee(vb, 10),
			"20": usdFee(vb, 20),
		},
	}
}

func usdFee(vbytes int, satPerVB int) float64 {
	const usdPerBTC = 26900.0
	return float64(vbytes*satPerVB) * usdPerBTC / 1e8
}

func readLinkedArtifact(path string) (linkedArtifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return linkedArtifact{}, err
	}
	var a linkedArtifact
	if err := json.Unmarshal(b, &a); err != nil {
		return linkedArtifact{}, err
	}
	return a, nil
}

func toMarkdown(r report) string {
	var sb strings.Builder
	sb.WriteString("# CRAB-He Evaluation Results (Generated)\n\n")
	sb.WriteString("Generated at: " + r.GeneratedAtUTC + "\n\n")
	sb.WriteString("Reference BTC price for fee conversion: $26,900/BTC (aligned with CRAB).\n\n")
	sb.WriteString("## 1) Transaction Table\n\n")
	sb.WriteString("| name_object | type | vbytes | fee_usd@2sat/vB | fee_usd@7sat/vB | fee_usd@10sat/vB | fee_usd@20sat/vB |\n")
	sb.WriteString("|---|---|---:|---:|---:|---:|---:|\n")
	for _, row := range r.TxTable {
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %.2f | %.2f | %.2f | %.2f |\n",
			row.Name, row.ObjectType, row.VBytes, row.FeeUSDBySatPerVB["2"], row.FeeUSDBySatPerVB["7"], row.FeeUSDBySatPerVB["10"], row.FeeUSDBySatPerVB["20"]))
	}

	sb.WriteString("\n## 2) Linked ACS On-chain Evidence\n\n")
	if len(r.LinkedDeployments) == 0 {
		sb.WriteString("No linked_acs artifacts found. Run deploy script first.\n")
	} else {
		sb.WriteString("| network | wallet | fund_txid | spend_txid | fund_vout | fund_value_sat | spend_value_sat | fee_sat | witness_order |\n")
		sb.WriteString("|---|---|---|---|---:|---:|---:|---:|---|\n")
		for _, a := range r.LinkedDeployments {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d | %d | %d | %d | %s |\n",
				a.Network, a.Wallet, a.FundTxID, a.SpendTxID, a.FundVout, a.FundValueSat, a.SpendValueSat, a.FeeSat, a.WitnessOrder))
		}
	}

	sb.WriteString("\n## 3) CLBA Summary\n\n")
	sb.WriteString(fmt.Sprintf("- crab_rational_width_sat: %s\n", r.CLBASummary.CRABRationalWidthSat))
	sb.WriteString(fmt.Sprintf("- crab_byzantine_width_sat: %s\n", r.CLBASummary.CRABByzantineWidthSat))
	sb.WriteString(fmt.Sprintf("- crab_he_width_sat: %s\n", r.CLBASummary.CRABHeWidthSat))
	sb.WriteString(fmt.Sprintf("- crab_he_infeasible: %t\n", r.CLBASummary.CRABHeInfeasible))
	sb.WriteString(fmt.Sprintf("- c_star_sat: %s\n", r.CLBASummary.CStarSat))

	return sb.String()
}

func sat(v *big.Int) int64 {
	return v.Int64()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
