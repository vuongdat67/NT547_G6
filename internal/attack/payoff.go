package attack

import "time"

// Profile fixes the analytical values used by the CLBA payoff mock.
// It models the side-deal decision, not a production miner marketplace.
type Profile struct {
	VSat     int64 `json:"vSat"`
	VDepSat  int64 `json:"vDepSat"`
	VColSat  int64 `json:"vColSat"`
	CSat     int64 `json:"cSat"`
	CStarSat int64 `json:"cStarSat"`
}

type DecisionReport struct {
	GeneratedAtUTC string     `json:"generatedAtUtc"`
	Scope          string     `json:"scope"`
	Profile        Profile    `json:"profile"`
	Decisions      []Decision `json:"decisions"`
	Notes          []string   `json:"notes"`
}

type Decision struct {
	Scheme              string `json:"scheme"`
	OfferedBRSat        int64  `json:"offeredBrSat"`
	BobUBSat            int64  `json:"bobUbSat"`
	MinerLBSat          int64  `json:"minerLbSat"`
	WidthSat            int64  `json:"widthSat"`
	MinerAccepts        bool   `json:"minerAccepts"`
	BobProfits          bool   `json:"bobProfits"`
	JointlyProfitable   bool   `json:"jointlyProfitable"`
	BobSurplusSat       int64  `json:"bobSurplusSat"`
	MinerSurplusSat     int64  `json:"minerSurplusSat"`
	DecisionRule        string `json:"decisionRule"`
	Reason              string `json:"reason"`
	SideDealDescription string `json:"sideDealDescription"`
}

func DefaultProfile() Profile {
	return Profile{
		VSat:     2_500_000,
		VDepSat:  500_000,
		VColSat:  500_000,
		CSat:     2_500_000,
		CStarSat: 3_000_000,
	}
}

func BuildDecisionReport() DecisionReport {
	p := DefaultProfile()
	return DecisionReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Scope:          "deterministic SDRBA-style payoff mock; no production miner bribery marketplace is implemented",
		Profile:        p,
		Decisions: []Decision{
			BaselineDecision("Naive CRAB+He", p.VSat, p.CSat, p.VDepSat, p.VColSat, 0),
			BaselineDecision("Collateral-only c'=2c", p.VSat, 2*p.CSat, p.VDepSat, p.VColSat, 0),
			CRABHeDecision("CRAB-He c*=v+v_dep-eps", p.VSat, p.VDepSat, p.VColSat, p.CStarSat-1_000, 0),
			CRABHeDecision("CRAB-He c*=v+v_dep", p.VSat, p.VDepSat, p.VColSat, p.CStarSat, 0),
			CRABHeDecision("CRAB-He c*=v+v_dep+eps", p.VSat, p.VDepSat, p.VColSat, p.CStarSat+1_000, 0),
		},
		Notes: []string{
			"Baseline uses Bob-UB = v+c+v_dep and Miner-LB = c+v_col.",
			"Collateral-only inflation preserves width because c' appears on both sides.",
			"CRAB-He uses Bob-UB = v+v_dep and Miner-LB = c; at c=v+v_dep the bribe interval is empty.",
		},
	}
}

func BaselineDecision(scheme string, vSat, cSat, vDepSat, vColSat, offeredBRSat int64) Decision {
	ub := vSat + cSat + vDepSat
	lb := cSat + vColSat
	return decide(scheme, ub, lb, offeredBRSat, "BR >= c+v_col and BR < v+c+v_dep", "Bob offers BR to one actively rational miner to censor CRAB revocation and HTLC honest paths.")
}

func CRABHeDecision(scheme string, vSat, vDepSat, vColSat, cSat, offeredBRSat int64) Decision {
	_ = vColSat
	ub := vSat + vDepSat
	lb := cSat
	return decide(scheme, ub, lb, offeredBRSat, "BR >= c and BR < v+v_dep", "Bob offers BR, but dep-B reveals pre_b and linked ACS removes c from Bob's transferable budget.")
}

func decide(scheme string, bobUBSat, minerLBSat, offeredBRSat int64, rule string, sideDeal string) Decision {
	width := bobUBSat - minerLBSat
	br := offeredBRSat
	if br == 0 && width > 0 {
		br = minerLBSat + width/2
	}

	minerAccepts := br >= minerLBSat && br > 0
	bobProfits := br < bobUBSat && br > 0
	joint := minerAccepts && bobProfits
	bobSurplus := bobUBSat - br
	minerSurplus := br - minerLBSat
	if br == 0 {
		bobSurplus = 0
		minerSurplus = 0
	}

	reason := "valid BR interval exists"
	if !joint {
		reason = "no jointly profitable BR interval"
	}
	return Decision{
		Scheme:              scheme,
		OfferedBRSat:        br,
		BobUBSat:            bobUBSat,
		MinerLBSat:          minerLBSat,
		WidthSat:            width,
		MinerAccepts:        minerAccepts,
		BobProfits:          bobProfits,
		JointlyProfitable:   joint,
		BobSurplusSat:       bobSurplus,
		MinerSurplusSat:     minerSurplus,
		DecisionRule:        rule,
		Reason:              reason,
		SideDealDescription: sideDeal,
	}
}
