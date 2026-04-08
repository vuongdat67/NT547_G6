// Package channel contains coalition-aware CRAB-He analysis helpers.
//
// This file provides an INTERPRETIVE DIAGNOSTIC for coalition-sized
// attacker groups, borrowing the He-HTLC Lemma 14 floor (k * v_col) rather
// than the composed CRAB-He single-miner floor c* used in Theorem 2 of the
// paper. The diagnostic exists to make artifact outputs readable against
// the original He-HTLC coalition convention; it is NOT a replacement for
// the composed single-miner security proof in Theorem 2.
//
// If you are looking for the CRAB-He composed security bound, see the
// single-miner CLBAAnalysis in params.go (BRLowerBoundLinked and
// BRUpperBoundLinked).
package channel

import (
	"fmt"
	"math"
	"math/big"
)

// CoalitionAnalysis models CLBA under a coalition of k actively rational
// miners using the He-HTLC standalone coalition convention (acceptance
// floor = k * v_col). This is an artifact-output interpretation aid, not
// a composed-model security bound.
type CoalitionAnalysis struct {
	Params  *Params
	K       int
	LambdaK float64
	// Fee is retained for API/report compatibility; coalition thresholds in this
	// implementation follow the SDRBA convention and do not subtract fee terms.
	Fee *big.Int
}

// NewCoalitionAnalysis creates a coalition CLBA diagnostic analysis under
// the He-HTLC standalone coalition convention. feeSat is retained for API
// compatibility and reporting output only.
func NewCoalitionAnalysis(p *Params, k int, lambdaK float64, feeSat int64) (*CoalitionAnalysis, error) {
	if p == nil {
		return nil, fmt.Errorf("params must not be nil")
	}
	if k < 1 {
		return nil, fmt.Errorf("coalition size k must be >= 1, got %d", k)
	}
	if lambdaK <= 0 || lambdaK >= 0.5 {
		return nil, fmt.Errorf("lambdaK must be in (0, 0.5), got %f", lambdaK)
	}
	return &CoalitionAnalysis{
		Params:  p,
		K:       k,
		LambdaK: lambdaK,
		Fee:     big.NewInt(feeSat),
	}, nil
}

// MinerLBCoalition returns the minimum total bribe the coalition will
// accept under the He-HTLC Lemma 14 convention, namely k * v_col. Note
// this is the STANDALONE He-HTLC coalition floor, not the composed
// CRAB-He single-miner floor c* used in Theorem 2.
func (a *CoalitionAnalysis) MinerLBCoalition() *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(a.K)), a.Params.VCol)
}

// BobUBLinked returns Bob's transferable bribe budget under linked ACS.
// This follows the paper definition UB_Bob^He = v + v_dep.
func (a *CoalitionAnalysis) BobUBLinked() *big.Int {
	return new(big.Int).Add(a.Params.V, a.Params.VDep)
}

// WidthCoalition returns the feasible range width for a coalition of size
// k under the He-HTLC standalone coalition convention. A negative value
// indicates the diagnostic rules out CLBA in that convention; it does NOT
// by itself imply composed-model security, which is governed by Theorem 2.
func (a *CoalitionAnalysis) WidthCoalition() *big.Int {
	return new(big.Int).Sub(a.BobUBLinked(), a.MinerLBCoalition())
}

// IsCLBAFeasibleCoalition returns true if the diagnostic coalition width
// is strictly positive. This reflects the He-HTLC standalone coalition
// convention only.
func (a *CoalitionAnalysis) IsCLBAFeasibleCoalition() bool {
	return a.WidthCoalition().Sign() > 0
}

// CStarForCoalition is a purely COMPARATIVE value exposed for diagnostic
// reporting. It is anchored at the single-miner burn-based bound
// c* = v + v_dep and reduced by (k-1) * v_col to visualize how the
// single-miner bound would shrink if one naively extrapolated the
// He-HTLC coalition floor into the CRAB-He composed model. This
// extrapolation is NOT derived from the composed Theorem 2 and must not
// be cited as an independent security threshold.
func (a *CoalitionAnalysis) CStarForCoalition() *big.Int {
	if a.K <= 1 {
		return new(big.Int).Set(a.Params.CStar)
	}
	reduction := new(big.Int).Mul(big.NewInt(int64(a.K-1)), a.Params.VCol)
	cStar := new(big.Int).Sub(new(big.Int).Set(a.Params.CStar), reduction)
	if cStar.Sign() < 0 {
		return big.NewInt(0)
	}
	return cStar
}

// KMax returns the diagnostic threshold index k where the He-HTLC
// standalone coalition floor first exceeds Bob's transferable upper bound
// v + v_dep. feeSat is kept for API compatibility and is ignored here.
func KMax(p *Params, feeSat int64) int {
	_ = feeSat
	if p == nil {
		return 0
	}
	if p.VCol.Sign() <= 0 {
		return math.MaxInt32
	}
	numerator := new(big.Int).Add(p.V, p.VDep)
	return int(new(big.Int).Div(numerator, p.VCol).Int64())
}

// Report generates a human-readable interpretive-diagnostic summary.
func (a *CoalitionAnalysis) Report() string {
	status := "DIAGNOSTIC-FEASIBLE (He-HTLC coalition convention)"
	if !a.IsCLBAFeasibleCoalition() {
		status = "DIAGNOSTIC-INFEASIBLE (He-HTLC coalition convention)"
	}
	return fmt.Sprintf(
		"=== Coalition Diagnostic (interpretive, k=%d) ===\n"+
			"  convention : He-HTLC Lemma 14 standalone coalition floor\n"+
			"  v          = %s sat\n"+
			"  v_dep      = %s sat\n"+
			"  v_col      = %s sat\n"+
			"  c*         = %s sat (composed-model single-miner bound)\n"+
			"  fee f      = %s sat (metadata only)\n"+
			"  Lambda_K   = %.3f\n"+
			"  Bob-UB     (v+v_dep)             = %s sat\n"+
			"  Miner-LB_k (k*v_col, diagnostic) = %s sat\n"+
			"  Width_k    (Bob-UB - Miner-LB_k) = %s sat\n"+
			"  c*_k comparative reference        = %s sat\n"+
			"  Diagnostic status: %s\n"+
			"  NOTE: composed-model security is governed by Theorem 2 (single-miner c*),\n"+
			"  not by this diagnostic. See Remark 2 in the paper.\n",
		a.K,
		a.Params.V, a.Params.VDep, a.Params.VCol,
		a.Params.CStar, a.Fee,
		a.LambdaK,
		a.BobUBLinked(),
		a.MinerLBCoalition(),
		a.WidthCoalition(),
		a.CStarForCoalition(),
		status,
	)
}
