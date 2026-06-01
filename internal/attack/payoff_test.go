package attack

import "testing"

func TestBaselineDecisionProfitableAtMidpoint(t *testing.T) {
	p := DefaultProfile()
	d := BaselineDecision("baseline", p.VSat, p.CSat, p.VDepSat, p.VColSat, 0)

	if d.WidthSat != p.VSat+p.VDepSat-p.VColSat {
		t.Fatalf("baseline width mismatch: got %d", d.WidthSat)
	}
	if !d.JointlyProfitable {
		t.Fatalf("baseline should have a profitable side-deal interval: %+v", d)
	}
	if d.OfferedBRSat <= d.MinerLBSat || d.OfferedBRSat >= d.BobUBSat {
		t.Fatalf("midpoint BR outside profitable interval: %+v", d)
	}
}

func TestCollateralOnlyInflationDoesNotChangeWidth(t *testing.T) {
	p := DefaultProfile()
	base := BaselineDecision("base", p.VSat, p.CSat, p.VDepSat, p.VColSat, 0)
	inflated := BaselineDecision("inflated", p.VSat, 2*p.CSat, p.VDepSat, p.VColSat, 0)

	if inflated.WidthSat != base.WidthSat {
		t.Fatalf("collateral-only width changed: base=%d inflated=%d", base.WidthSat, inflated.WidthSat)
	}
	if !inflated.JointlyProfitable {
		t.Fatalf("collateral-only inflated baseline should remain profitable: %+v", inflated)
	}
}

func TestCRABHeBoundaryAtCStar(t *testing.T) {
	p := DefaultProfile()

	below := CRABHeDecision("below", p.VSat, p.VDepSat, p.VColSat, p.CStarSat-1_000, 0)
	at := CRABHeDecision("at", p.VSat, p.VDepSat, p.VColSat, p.CStarSat, 0)
	above := CRABHeDecision("above", p.VSat, p.VDepSat, p.VColSat, p.CStarSat+1_000, 0)

	if !below.JointlyProfitable || below.WidthSat != 1_000 {
		t.Fatalf("c<c* should leave a positive interval: %+v", below)
	}
	if at.JointlyProfitable || at.WidthSat != 0 {
		t.Fatalf("c=c* should close the interval: %+v", at)
	}
	if above.JointlyProfitable || above.WidthSat >= 0 {
		t.Fatalf("c>c* should keep the interval closed: %+v", above)
	}
}

func TestExplicitBoundaryOffers(t *testing.T) {
	p := DefaultProfile()
	atLB := BaselineDecision("at-lb", p.VSat, p.CSat, p.VDepSat, p.VColSat, p.CSat+p.VColSat)
	atUB := BaselineDecision("at-ub", p.VSat, p.CSat, p.VDepSat, p.VColSat, p.VSat+p.CSat+p.VDepSat)

	if !atLB.MinerAccepts || !atLB.BobProfits {
		t.Fatalf("BR at lower bound should be accepted and still below Bob UB: %+v", atLB)
	}
	if atUB.BobProfits || atUB.JointlyProfitable {
		t.Fatalf("BR at upper bound should not be profitable for Bob: %+v", atUB)
	}
}
