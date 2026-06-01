package experiments

import "time"

// AttackTimelineReport records a deterministic end-to-end CLBA replay at the
// analytical profile used in the paper tables.
type AttackTimelineReport struct {
	GeneratedAtUTC string             `json:"generatedAtUtc"`
	Profile        AttackProfile      `json:"profile"`
	Baseline       AttackScenario     `json:"baselineCrabPlusHe"`
	CRABHe         AttackScenario     `json:"crabHe"`
	Rows           []AttackSummaryRow `json:"rows"`
	Notes          []string           `json:"notes"`
}

type AttackProfile struct {
	VSat            int64 `json:"vSat"`
	VDepSat         int64 `json:"vDepSat"`
	VColSat         int64 `json:"vColSat"`
	BaselineCSat    int64 `json:"baselineCSat"`
	CRABHeCStarSat  int64 `json:"crabHeCStarSat"`
	Kappa           int   `json:"kappa"`
	LinkedFeeSat    int64 `json:"linkedFeeSat"`
	LinkedBurnSat   int64 `json:"linkedBurnSat"`
	RegtestCommitVB int   `json:"regtestCommitVb"`
	LinkedSpendVB   int   `json:"linkedSpendVb"`
}

type AttackScenario struct {
	Scheme          string        `json:"scheme"`
	BobUBSat        int64         `json:"bobUbSat"`
	MinerLBSat      int64         `json:"minerLbSat"`
	WidthSat        int64         `json:"widthSat"`
	SelectedBRSat   int64         `json:"selectedBrSat"`
	Profitable      bool          `json:"profitable"`
	BobSurplusSat   int64         `json:"bobSurplusSat"`
	MinerSurplusSat int64         `json:"minerSurplusSat"`
	Outcome         string        `json:"outcome"`
	Events          []AttackEvent `json:"events"`
}

type AttackEvent struct {
	Phase       int    `json:"phase"`
	BlockWindow string `json:"blockWindow"`
	Actor       string `json:"actor"`
	Action      string `json:"action"`
	Effect      string `json:"effect"`
}

type AttackSummaryRow struct {
	Scheme        string `json:"scheme"`
	MinerLBSat    int64  `json:"minerLbSat"`
	BobUBSat      int64  `json:"bobUbSat"`
	WidthSat      int64  `json:"widthSat"`
	SelectedBRSat int64  `json:"selectedBrSat"`
	Profitable    bool   `json:"profitable"`
	Outcome       string `json:"outcome"`
}

func BuildAttackTimelineReport() AttackTimelineReport {
	profile := AttackProfile{
		VSat:            2_500_000,
		VDepSat:         500_000,
		VColSat:         500_000,
		BaselineCSat:    2_500_000,
		CRABHeCStarSat:  3_000_000,
		Kappa:           3,
		LinkedFeeSat:    500_000,
		LinkedBurnSat:   2_500_000,
		RegtestCommitVB: 281,
		LinkedSpendVB:   246,
	}

	baseline := buildBaselineAttackScenario(profile)
	crabHe := buildCRABHeAttackScenario(profile)
	rows := []AttackSummaryRow{
		scenarioRow(baseline),
		scenarioRow(crabHe),
	}

	return AttackTimelineReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Profile:        profile,
		Baseline:       baseline,
		CRABHe:         crabHe,
		Rows:           rows,
		Notes: []string{
			"The replay is a deterministic incentive-model simulation, not a mainnet incident claim.",
			"Baseline width follows W = v + v_dep - v_col.",
			"CRAB-He width follows W' = v + v_dep - c_star and is zero at c_star = v + v_dep.",
		},
	}
}

func buildBaselineAttackScenario(p AttackProfile) AttackScenario {
	ub := p.VSat + p.BaselineCSat + p.VDepSat
	lb := p.BaselineCSat + p.VColSat
	width := ub - lb
	br := lb + width/2
	return AttackScenario{
		Scheme:          "Naive CRAB+He",
		BobUBSat:        ub,
		MinerLBSat:      lb,
		WidthSat:        width,
		SelectedBRSat:   br,
		Profitable:      width > 0,
		BobSurplusSat:   ub - br,
		MinerSurplusSat: br - lb,
		Outcome:         "profitable CLBA interval exists; stale-state and HTLC-side gains fund the bribe",
		Events: []AttackEvent{
			{Phase: 0, BlockWindow: "off-chain before t_pub", Actor: "Bob", Action: "offers BR to one actively rational miner", Effect: "BR satisfies c+v_col < BR < v+c+v_dep"},
			{Phase: 1, BlockWindow: "after stale commitment appears", Actor: "Miner", Action: "censors tx_revoke_ACS", Effect: "Alice's Sleepy CRAB punishment path is delayed"},
			{Phase: 2, BlockWindow: "before HTLC timeout T", Actor: "Miner", Action: "censors Alice's tx_dep_A", Effect: "Alice cannot claim v_dep through the honest HTLC path"},
			{Phase: 3, BlockWindow: "after timeout T", Actor: "Bob", Action: "spends stale channel state and HTLC timeout paths", Effect: "Bob keeps positive surplus after paying BR"},
		},
	}
}

func buildCRABHeAttackScenario(p AttackProfile) AttackScenario {
	ub := p.VSat + p.VDepSat
	lb := p.CRABHeCStarSat
	width := ub - lb
	return AttackScenario{
		Scheme:          "CRAB-He",
		BobUBSat:        ub,
		MinerLBSat:      lb,
		WidthSat:        width,
		SelectedBRSat:   0,
		Profitable:      width > 0,
		BobSurplusSat:   0,
		MinerSurplusSat: 0,
		Outcome:         "no jointly profitable BR exists; dep-B reveals pre_b and triggers fixed linked burn/fee spend",
		Events: []AttackEvent{
			{Phase: 0, BlockWindow: "off-chain before t_pub", Actor: "Bob", Action: "searches for a profitable BR", Effect: "requires BR < v+v_dep and BR >= c_star, an empty interval"},
			{Phase: 1, BlockWindow: "if stale commitment is attempted", Actor: "Bob", Action: "must still use dep-B to obtain HTLC-side value", Effect: "pre_b becomes public on chain"},
			{Phase: 2, BlockWindow: "kappa-window after dep-B", Actor: "Any non-colluding miner", Action: "includes pre-signed linked ACS spend", Effect: "miner earns v_col as fee and c_star-v_col is burned"},
			{Phase: 3, BlockWindow: "terminal state", Actor: "Bob", Action: "cannot redirect linked output to himself", Effect: "CLBA self-defeats under c_star = v+v_dep"},
		},
	}
}

func scenarioRow(s AttackScenario) AttackSummaryRow {
	return AttackSummaryRow{
		Scheme:        s.Scheme,
		MinerLBSat:    s.MinerLBSat,
		BobUBSat:      s.BobUBSat,
		WidthSat:      s.WidthSat,
		SelectedBRSat: s.SelectedBRSat,
		Profitable:    s.Profitable,
		Outcome:       s.Outcome,
	}
}
