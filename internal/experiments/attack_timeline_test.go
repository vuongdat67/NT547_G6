package experiments

import "testing"

func TestAttackTimelineMatchesAnalyticalProfile(t *testing.T) {
	rep := BuildAttackTimelineReport()

	if rep.Profile.CRABHeCStarSat != rep.Profile.VSat+rep.Profile.VDepSat {
		t.Fatalf("c_star mismatch: got %d", rep.Profile.CRABHeCStarSat)
	}
	if got := rep.Baseline.WidthSat; got != rep.Profile.VSat+rep.Profile.VDepSat-rep.Profile.VColSat {
		t.Fatalf("baseline width mismatch: got %d", got)
	}
	if !rep.Baseline.Profitable {
		t.Fatalf("baseline CLBA should be profitable")
	}
	if rep.CRABHe.WidthSat != 0 {
		t.Fatalf("CRAB-He width should be zero at c_star, got %d", rep.CRABHe.WidthSat)
	}
	if rep.CRABHe.Profitable {
		t.Fatalf("CRAB-He CLBA should be infeasible at c_star")
	}
}
