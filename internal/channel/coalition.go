// Package channel contains coalition-aware CRAB-He analysis helpers.
package channel

import (
	"fmt"
	"math"
	"math/big"
)

// CoalitionAnalysis models CLBA under a coalition of k actively rational miners.
type CoalitionAnalysis struct {
	Params  *Params
	K       int
	LambdaK float64
	// Fee is retained for API/report compatibility; coalition thresholds in this
	// implementation follow the SDRBA convention and do not subtract fee terms.
	Fee *big.Int
}

// NewCoalitionAnalysis creates a coalition CLBA analysis.
// feeSat is retained for API compatibility and reporting output.
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

// MinerLBCoalition returns the minimum total bribe the coalition will accept.
// Consistent with CLBAAnalysis.BRLowerBoundLinked(), each miner threshold is v_col
// under the SDRBA coinbase model, so coalition threshold is k*v_col.
func (a *CoalitionAnalysis) MinerLBCoalition() *big.Int {
	return new(big.Int).Mul(big.NewInt(int64(a.K)), a.Params.VCol)
}

// BobUBLinked returns Bob's max bribe budget under linked ACS.
func (a *CoalitionAnalysis) BobUBLinked() *big.Int {
	ub := new(big.Int).Add(a.Params.V, a.Params.VDep)
	ub.Sub(ub, a.Params.CStar)
	if ub.Sign() < 0 {
		return big.NewInt(0)
	}
	return ub
}

// WidthCoalition returns the feasible range width for a coalition of size k.
func (a *CoalitionAnalysis) WidthCoalition() *big.Int {
	return new(big.Int).Sub(a.BobUBLinked(), a.MinerLBCoalition())
}

// IsCLBAFeasibleCoalition returns true if the coalition attack succeeds.
func (a *CoalitionAnalysis) IsCLBAFeasibleCoalition() bool {
	return a.WidthCoalition().Sign() > 0
}

// CStarForCoalition returns the minimum c* needed to block a coalition of size k.
// c*_k = v + v_dep - k*v_col.
func (a *CoalitionAnalysis) CStarForCoalition() *big.Int {
	kVCol := new(big.Int).Mul(big.NewInt(int64(a.K)), a.Params.VCol)
	cStar := new(big.Int).Add(a.Params.V, a.Params.VDep)
	cStar.Sub(cStar, kVCol)
	if cStar.Sign() < 0 {
		return big.NewInt(0)
	}
	return cStar
}

// KMax returns the threshold index k where coalition infeasibility begins for c*=0.
// feeSat is kept for API compatibility and is ignored in this model.
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

// Report generates a human-readable analysis summary.
func (a *CoalitionAnalysis) Report() string {
	status := "PROFITABLE (coalition attack succeeds)"
	if !a.IsCLBAFeasibleCoalition() {
		status = "INFEASIBLE (defense holds)"
	}
	return fmt.Sprintf(
		"=== Coalition CLBA Analysis (k=%d miners) ===\n"+
			"  v          = %s sat\n"+
			"  v_dep      = %s sat\n"+
			"  v_col      = %s sat\n"+
			"  c*         = %s sat\n"+
			"  fee f      = %s sat\n"+
			"  Lambda_K   = %.3f\n"+
			"  Bob-UB     (v+v_dep-c*)          = %s sat\n"+
			"  Miner-LB_k (k*v_col)             = %s sat\n"+
			"  Width_k    (Bob-UB - Miner-LB_k) = %s sat\n"+
			"  c*_k needed to block k-coalition = %s sat\n"+
			"  Coalition CLBA: %s\n",
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
