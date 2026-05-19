package channel_test

import (
	"math/big"
	"testing"

	"github.com/crab-he/internal/channel"
)

func testParams(t *testing.T) *channel.Params {
	t.Helper()
	v := big.NewInt(2_500_000)
	vDep := big.NewInt(500_000)
	vCol := big.NewInt(500_000)
	delta := big.NewInt(1_000)
	p, err := channel.NewParams(v, vDep, vCol, delta, 144, 288, 6, 3)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSingleMinerMatchesExisting(t *testing.T) {
	p := testParams(t)
	ca, err := channel.NewCoalitionAnalysis(p, 1, 0.3, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := ca.CStarForCoalition()
	want := p.CStar
	if got.Cmp(want) != 0 {
		t.Fatalf("k=1 c*_1 = %s, want %s", got, want)
	}
	expectedWidth := new(big.Int).Add(p.V, p.VDep)
	expectedWidth.Sub(expectedWidth, p.VCol)
	if w := ca.WidthCoalition(); w.Cmp(expectedWidth) != 0 {
		t.Fatalf("k=1 width = %s, want %s", w, expectedWidth)
	}
	if !ca.IsCLBAFeasibleCoalition() {
		t.Fatal("k=1 width should be positive and therefore feasible")
	}
}

func TestCoalitionWidthDecreases(t *testing.T) {
	p := testParams(t)
	prev := new(big.Int).SetInt64(1 << 62)
	for k := 1; k <= 5; k++ {
		ca, err := channel.NewCoalitionAnalysis(p, k, 0.05, 1000)
		if err != nil {
			t.Fatal(err)
		}
		w := ca.WidthCoalition()
		if k > 1 && w.Cmp(prev) >= 0 {
			t.Fatalf("k=%d: width %s should be < previous %s", k, w, prev)
		}
		prev = new(big.Int).Set(w)
	}
}

func TestKMaxThreshold(t *testing.T) {
	p := testParams(t)
	feeSat := int64(1000)
	kMax := channel.KMax(p, feeSat)
	if kMax <= 0 {
		t.Fatal("kMax must be positive")
	}

	ca, err := channel.NewCoalitionAnalysis(p, kMax, 0.49, feeSat)
	if err != nil {
		t.Fatal(err)
	}
	if ca.IsCLBAFeasibleCoalition() {
		t.Fatalf("k=%d (kMax): expected infeasible, got feasible; width=%s", kMax, ca.WidthCoalition())
	}

	if kMax > 1 {
		ca2, err := channel.NewCoalitionAnalysis(p, kMax-1, 0.49, feeSat)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("k=%d: width=%s feasible=%v", kMax-1, ca2.WidthCoalition(), ca2.IsCLBAFeasibleCoalition())
	}
}

func TestCStarKDecreasesWithK(t *testing.T) {
	p := testParams(t)
	feeSat := int64(1000)
	prev := new(big.Int).Set(p.CStar)
	hitZero := false
	for k := 2; k <= 7; k++ {
		ca, err := channel.NewCoalitionAnalysis(p, k, 0.05, feeSat)
		if err != nil {
			t.Fatal(err)
		}
		cStarK := ca.CStarForCoalition()
		if cStarK.Cmp(prev) > 0 {
			t.Fatalf("k=%d: c*_k=%s should be <= previous=%s", k, cStarK, prev)
		}
		if !hitZero && cStarK.Sign() == 0 {
			hitZero = true
		}
		prev = new(big.Int).Set(cStarK)
	}
	if !hitZero {
		t.Fatal("expected c*_k to reach zero for sufficiently large k")
	}
}
