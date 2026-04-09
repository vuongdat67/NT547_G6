// Package channel implements CRAB-He channel construction.
// Based on CRAB (CCS'24) by Aumayr et al. and He-HTLC (NDSS'23) by Wadhwa et al.
// Extended with Cross-Layer Bribery Attack (CLBA) defense via linked revocation.
package channel

import (
	"crypto/sha256"
	"fmt"
	"math/big"
)

// Params holds CRAB-He channel parameters.
type Params struct {
	V         *big.Int
	CStar     *big.Int
	Delta     *big.Int
	T         int64
	VDep      *big.Int
	VCol      *big.Int
	AbsoluteT int64
	Ell       int64
	Kappa     int
}

// NewParams creates CRAB-He parameters with collateral derived from the
// burn-based linked-ACS game: c* = v + v_dep.
func NewParams(v, vDep, vCol, delta *big.Int, t, absT, ell int64, kappa int) (*Params, error) {
	if kappa <= 2 {
		return nil, fmt.Errorf("kappa must be > 2, got %d", kappa)
	}
	minVCol := new(big.Int).Div(vDep, big.NewInt(int64(kappa-1)))
	if vCol.Cmp(minVCol) < 0 {
		return nil, fmt.Errorf("v_col=%s < v_dep/(kappa-1)=%s, violates He-HTLC Theorem 3", vCol, minVCol)
	}
	if vCol.Cmp(vDep) > 0 {
		return nil, fmt.Errorf("v_col=%s > v_dep=%s, violates He-HTLC Theorem 3", vCol, vDep)
	}
	if absT <= t {
		return nil, fmt.Errorf("absolute HTLC timeout %d must exceed channel revocation window %d", absT, t)
	}
	if ell < 1 {
		return nil, fmt.Errorf("ell must be >= 1, got %d", ell)
	}

	cStar := new(big.Int).Add(v, vDep)

	return &Params{
		V:         v,
		CStar:     cStar,
		Delta:     delta,
		T:         t,
		VDep:      vDep,
		VCol:      vCol,
		AbsoluteT: absT,
		Ell:       ell,
		Kappa:     kappa,
	}, nil
}

func (p *Params) VerifyCLBAInfeasible() bool {
	lhs := new(big.Int).Add(p.V, p.VDep)
	return p.CStar.Cmp(lhs) >= 0
}

func (p *Params) OverheadAboveCRABByzantine() *big.Int {
	return new(big.Int).Sub(p.CStar, p.V)
}

func (p *Params) String() string {
	return fmt.Sprintf(
		"CRAB-He Params:\n"+
			"  v       = %s sat\n"+
			"  c*      = %s sat  (v+v_dep)\n"+
			"  v_dep   = %s sat\n"+
			"  v_col   = %s sat\n"+
			"  overhead= %s sat above CRAB-Byzantine\n"+
			"  CLBA infeasible: %v\n"+
			"  T (channel) = %d blocks\n"+
			"  T (HTLC)   = %d block height\n"+
			"  ell        = %d blocks\n"+
			"  kappa      = %d distinct miners\n",
		p.V, p.CStar, p.VDep, p.VCol,
		p.OverheadAboveCRABByzantine(),
		p.VerifyCLBAInfeasible(),
		p.T, p.AbsoluteT, p.Ell, p.Kappa,
	)
}

type RevocationSecret struct {
	Secret []byte
	Hash   []byte
	State  int
}

func NewRevocationSecret(secret []byte, state int) *RevocationSecret {
	h := sha256.Sum256(secret)
	return &RevocationSecret{Secret: secret, Hash: h[:], State: state}
}

type HTLCSecrets struct {
	PreA     []byte
	PreB     []byte
	HashPreA []byte
	HashPreB []byte
}

func NewHTLCSecrets(preA, preB []byte) *HTLCSecrets {
	hA := sha256.Sum256(preA)
	hB := sha256.Sum256(preB)
	return &HTLCSecrets{PreA: preA, PreB: preB, HashPreA: hA[:], HashPreB: hB[:]}
}

type CLBAAnalysis struct {
	Params      *Params
	LambdaI     float64
	CollateralC *big.Int
}

func NewCLBAAnalysis(p *Params, lambdaI float64, c *big.Int) (*CLBAAnalysis, error) {
	if lambdaI >= 0.5 || lambdaI <= 0 {
		return nil, fmt.Errorf("lambda_i must be in (0, 0.5), got %f", lambdaI)
	}
	return &CLBAAnalysis{Params: p, LambdaI: lambdaI, CollateralC: c}, nil
}

func (a *CLBAAnalysis) BRLowerBound() *big.Int {
	return new(big.Int).Add(a.CollateralC, a.Params.VCol)
}

func (a *CLBAAnalysis) BRUpperBound() *big.Int {
	ub := new(big.Int).Add(a.Params.V, a.CollateralC)
	ub.Add(ub, a.Params.VDep)
	return ub
}

// Width returns baseline CLBA width under the non-linked CRAB composition model:
//   W = (v + c + v_dep) - (c + v_col) = v + v_dep - v_col.
// This quantity keeps v_col because miner honest-path utility includes col-M.
func (a *CLBAAnalysis) Width() *big.Int {
	w := new(big.Int).Add(a.Params.V, a.Params.VDep)
	w.Sub(w, a.Params.VCol)
	return w
}

// BRLowerBoundLinked returns miner's minimum bribe for linked-ACS model.
// Honest path utility is c + v_col (CRAB punishment plus HTLC col-M), while CLBA-path
// utility is v_col + BR once dep-B reveals pre_b and linked ACS is claimed. Therefore
// BR must cover at least c.
func (a *CLBAAnalysis) BRLowerBoundLinked() *big.Int {
	return new(big.Int).Set(a.CollateralC)
}

// BRUpperBoundLinked returns Bob's max bribe in linked-ACS model.
// Bob's CLBA branch gross gain is v + c + v_dep, but revealing pre_b deterministically
// triggers loss of the linked output value c (burned via fixed linked spend template).
// Net transferable surplus is v + v_dep.
func (a *CLBAAnalysis) BRUpperBoundLinked() *big.Int {
	return new(big.Int).Add(a.Params.V, a.Params.VDep)
}

// WidthLinked returns BR range width in linked-ACS model:
//   W' = (v + v_dep) - c.
// Semantic shift from Width(): v_col disappears because miner indifference is
// evaluated at attacker-favorable linked-fee capture (pi=1), where v_col terms
// cancel symmetrically in the linked model.
// Width <= 0 implies CLBA is infeasible.
func (a *CLBAAnalysis) WidthLinked() *big.Int {
	w := a.BRUpperBoundLinked()
	w.Sub(w, a.BRLowerBoundLinked())
	return w
}

func (a *CLBAAnalysis) IsCLBAProfitableLinked() bool {
	return a.BRLowerBoundLinked().Cmp(a.BRUpperBoundLinked()) < 0
}

func (a *CLBAAnalysis) IsCLBAProfitable() bool {
	return a.BRLowerBound().Cmp(a.BRUpperBound()) < 0
}

func (a *CLBAAnalysis) Report() string {
	status := "PROFITABLE (attack succeeds)"
	if !a.IsCLBAProfitable() {
		status = "INFEASIBLE (defense holds)"
	}
	midBR := new(big.Int).Add(a.BRLowerBound(), a.BRUpperBound())
	midBR.Div(midBR, big.NewInt(2))

	return fmt.Sprintf(
		"=== CLBA Analysis ===\n"+
			"  v       = %s sat\n"+
			"  v_dep   = %s sat\n"+
			"  v_col   = %s sat\n"+
			"  c       = %s sat\n"+
			"  lambda_i= %.3f\n"+
			"  Miner-LB (c+v_col)        = %s sat\n"+
			"  Bob-UB   (v+c+v_dep)      = %s sat\n"+
			"  Width    (v+v_dep-v_col)  = %s sat\n"+
			"  Midpoint BR               = %s sat\n"+
			"  CLBA: %s\n",
		a.Params.V, a.Params.VDep, a.Params.VCol,
		a.CollateralC, a.LambdaI,
		a.BRLowerBound(), a.BRUpperBound(),
		a.Width(), midBR, status,
	)
}

// ReportLinked reports CLBA feasibility under CRAB-He linked ACS.
func (a *CLBAAnalysis) ReportLinked() string {
	status := "PROFITABLE (attack succeeds)"
	if !a.IsCLBAProfitableLinked() {
		status = "INFEASIBLE (defense holds)"
	}
	midBR := new(big.Int).Add(a.BRLowerBoundLinked(), a.BRUpperBoundLinked())
	midBR.Div(midBR, big.NewInt(2))

	return fmt.Sprintf(
		"=== CLBA Analysis (Linked ACS) ===\n"+
			"  v       = %s sat\n"+
			"  v_dep   = %s sat\n"+
			"  v_col   = %s sat\n"+
			"  c       = %s sat\n"+
			"  lambda_i= %.3f\n"+
			"  Miner-LB linked (c)          = %s sat\n"+
			"  Bob-UB   linked (v+v_dep)    = %s sat\n"+
			"  Width    linked              = %s sat\n"+
			"  Midpoint BR                  = %s sat\n"+
			"  CLBA linked: %s\n",
		a.Params.V, a.Params.VDep, a.Params.VCol,
		a.CollateralC, a.LambdaI,
		a.BRLowerBoundLinked(), a.BRUpperBoundLinked(),
		a.WidthLinked(), midBR, status,
	)
}
