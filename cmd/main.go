// CRAB-He evaluation entry point.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	"github.com/crab-he/internal/channel"
	"github.com/crab-he/internal/htlc"
)

func main() {
	fmt.Println("=== CRAB-He Evaluation ===")
	fmt.Println("Securing Lightning Payment Channels Against Actively Rational Miners")
	fmt.Println()

	fmt.Println("--- Step 1: Parameter Setup ---")
	// Analytical profile used by the paper tables and experiment artifacts.
	v := big.NewInt(2_500_000)
	vDep := big.NewInt(500_000)
	vCol := big.NewInt(500_000)
	delta := big.NewInt(1_000)

	var tChannel int64 = 144
	var tHTLC int64 = 288
	var ell int64 = 6
	kappa := 3

	params, err := channel.NewParams(v, vDep, vCol, delta, tChannel, tHTLC, ell, kappa)
	if err != nil {
		panic(fmt.Sprintf("param error: %v", err))
	}
	fmt.Println(params)

	fmt.Println("--- Step 2: CLBA Analysis ---")
	cCRAB := new(big.Int).Div(v, big.NewInt(2))
	attackAnalysis, _ := channel.NewCLBAAnalysis(params, 0.3, cCRAB)
	fmt.Println("[CRAB c=v/2]: " + attackAnalysis.Report())

	cByz := new(big.Int).Set(v)
	byzAnalysis, _ := channel.NewCLBAAnalysis(params, 0.3, cByz)
	fmt.Println("[CRAB Byzantine c=v]: " + byzAnalysis.Report())

	defenseAnalysis, _ := channel.NewCLBAAnalysis(params, 0.3, params.CStar)
	fmt.Println("[CRAB-He c*=v+v_dep]: " + defenseAnalysis.ReportLinked())

	fmt.Println("--- Step 3: Key Generation ---")
	alicePK := randomHex(32)
	bobPK := randomHex(32)
	fmt.Printf("Alice PK: %s...\n", alicePK[:16])
	fmt.Printf("Bob   PK: %s...\n", bobPK[:16])

	revA0 := channel.NewRevocationSecret(randomBytes(32), 0)
	revB0 := channel.NewRevocationSecret(randomBytes(32), 0)
	fmt.Printf("RevA[0]: H=%s...\n", fmt.Sprintf("%x", revA0.Hash)[:12])
	fmt.Printf("RevB[0]: H=%s...\n", fmt.Sprintf("%x", revB0.Hash)[:12])
	fmt.Println()

	fmt.Println("--- Step 4: HTLC Secrets ---")
	preA := randomBytes(32)
	preB := randomBytes(32)
	htlcSecrets := channel.NewHTLCSecrets(preA, preB)
	fmt.Printf("pre_a: %x...\n", preA[:6])
	fmt.Printf("pre_b: %x...  (shared only H(pre_b) with Alice for linked ACS)\n", preB[:6])
	fmt.Printf("H(pre_a): %s...\n", fmt.Sprintf("%x", htlcSecrets.HashPreA)[:12])
	fmt.Printf("H(pre_b): %s...\n", fmt.Sprintf("%x", htlcSecrets.HashPreB)[:12])
	fmt.Println()

	fmt.Println("--- Step 5: Channel Open ---")
	ch := channel.NewChannel(params, sat(v), 0, revA0, revB0, alicePK, bobPK)
	fmt.Print(channel.MakeFunding(params, alicePK, bobPK))
	commitA0NoHTLC := ch.GenerateCommitA()
	fmt.Print(commitA0NoHTLC)
	fmt.Println()

	fmt.Println("--- Step 6: Attach He-HTLC to Channel ---")
	ch.AttachHTLC(htlcSecrets)
	commitA0HTLC := ch.GenerateCommitA()
	fmt.Print(commitA0HTLC)
	sizeNoHTLC := commitA0NoHTLC.SizeVB
	fmt.Printf("Overhead vs CRAB: +%d vB (+%d%%)\n", commitA0HTLC.SizeVB-sizeNoHTLC, (commitA0HTLC.SizeVB-sizeNoHTLC)*100/sizeNoHTLC)
	fmt.Println()

	fmt.Println("--- Step 7: He-HTLC Scripts ---")
	he := htlc.NewHTLC(sat(params.VDep), sat(params.VCol), htlcSecrets.HashPreA, htlcSecrets.HashPreB, tHTLC, ell, alicePK, bobPK)
	he.Dep.TxID = "9f7f95a7f618feab0b0580d4eafca9e4b7d1faa49946e336cb88fdde6d9421a2"
	he.Col.TxID = "9f7f95a7f618feab0b0580d4eafca9e4b7d1faa49946e336cb88fdde6d9421a2"
	fmt.Println(he.Scripts())
	fmt.Println()

	fmt.Println("--- Step 8: Honest Execution (dep-A path) ---")
	fmt.Print(htlc.DepA(he.Dep, preA))
	ch.DetachHTLC()
	fmt.Println()

	fmt.Println("--- Step 9: CLBA Attempt Scenario ---")
	fmt.Println("Scenario: Bob posts old state and later broadcasts dep-B")
	ch.AttachHTLC(htlcSecrets)
	for _, tx := range ch.GeneratePunishmentBundle(0, revA0) {
		fmt.Print(tx)
	}
	fmt.Print(htlc.DepB(he.Dep, preB))
	fmt.Println(he.DepBTriggerNote())
	ch.DetachHTLC()
	fmt.Println()

	fmt.Println("--- Step 10: Linked ACS Execution ---")
	linkedACS := channel.MakeRevokeACSLinked(0, revA0, htlcSecrets, sat(params.CStar), sat(params.VCol))
	fmt.Print(linkedACS)
	burn := sat(params.CStar) - sat(params.VCol)
	if burn < 0 {
		burn = 0
	}
	fmt.Printf("\nMiner payout (linked path) = v_col = %d sat = %.4f BTC\n", sat(params.VCol), float64(sat(params.VCol))/1e8)
	fmt.Printf("Residual burn from linked output = %d sat\n", burn)
	fmt.Println("-> Bob cannot extract linked output surplus -> CLBA self-defeating")
	fmt.Println()

	fmt.Println("--- Step 11: Evaluation Summary ---")
	printEvaluationTable(params, commitA0NoHTLC.SizeVB, commitA0HTLC.SizeVB)
}

func printEvaluationTable(p *channel.Params, commitNoHTLCSize, commitWithHTLCSize int) {
	fmt.Printf("\n%-30s %6s %8s\n", "Transaction", "vBytes", "USD@7sat/vB")
	fmt.Println(strings.Repeat("-", 50))

	type row struct {
		label string
		vb    int
	}
	rows := []row{
		{"tx_fund (open)", 338},
		{"tx_commit_A (no HTLC)", commitNoHTLCSize},
		{"tx_commit_A (HTLC+linked ACS)", commitWithHTLCSize},
		{"tx_spend_A (honest close)", 418},
		{"tx_revoke_B (punishment B)", 192},
		{"tx_revoke_ACS_std (punishment miner)", 192},
		{"tx_revoke_ACS_linked (CRAB-He)", 246},
		{"tx_dep_A (He-HTLC dep-A)", 190},
		{"tx_dep_B (He-HTLC dep-B)", 172},
		{"tx_col_B (He-HTLC col-B)", 152},
		{"tx_col_M (He-HTLC col-M)", 168},
	}

	const satPerByte = 7
	const usdPerBTC = 26_900.0

	for _, r := range rows {
		usd := float64(r.vb) * satPerByte * usdPerBTC / 1e8
		fmt.Printf("  %-30s %6d  $%.2f\n", r.label, r.vb, usd)
	}

	fmt.Println()
	fmt.Printf("Collateral comparison (v=%d sat):\n", sat(p.V))
	fmt.Printf("  CRAB rational  c = v/2    = %d sat (%.4f BTC)\n", sat(p.V)/2, float64(sat(p.V)/2)/1e8)
	fmt.Printf("  CRAB Byzantine c = v      = %d sat (%.4f BTC)\n", sat(p.V), float64(sat(p.V))/1e8)
	fmt.Printf("  CRAB-He        c*=v+v_dep       = %d sat (%.4f BTC)\n", sat(p.CStar), float64(sat(p.CStar))/1e8)
	fmt.Printf("  Overhead vs CRAB Byzantine: +%d sat (+%.1f%%)\n", sat(p.OverheadAboveCRABByzantine()), float64(sat(p.OverheadAboveCRABByzantine()))*100/float64(sat(p.V)))
	fmt.Println()

	fmt.Println("(Single deterministic pass; no duplicated rows)")
	fmt.Println("n-hop collateral c*_n = v + n*v_dep:")
	for n := 1; n <= 7; n++ {
		cN := sat(p.V) + int64(n)*sat(p.VDep)
		pct := float64(cN-sat(p.V)) * 100 / float64(sat(p.V))
		fmt.Printf("  n=%d: c*_%d = %d sat  (+%.1f%% vs CRAB Byz)\n", n, n, cN, pct)
	}

	// Practical Lightning-like scenario (small HTLC relative to channel capacity).
	fmt.Println()
	fmt.Println("n-hop practical example (v_dep=5% of v):")
	vPrac := sat(p.V)
	vDepPrac := vPrac / 20 // 5%
	for n := 1; n <= 7; n++ {
		cN := vPrac + int64(n)*vDepPrac
		pct := float64(cN-vPrac) * 100 / float64(vPrac)
		fmt.Printf("  n=%d: c*_%d = %d sat  (+%.1f%% vs CRAB Byz)\n", n, n, cN, pct)
	}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func randomHex(n int) string {
	h := sha256.Sum256(randomBytes(32))
	s := fmt.Sprintf("%x", h)
	if n > len(s) {
		return s
	}
	return s[:n]
}

func sat(v *big.Int) int64 { return v.Int64() }
