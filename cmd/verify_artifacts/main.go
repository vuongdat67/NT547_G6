package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crab-he/internal/attack"
	"github.com/crab-he/internal/experiments"
)

func main() {
	base := filepath.Join("artifacts", "experiments")
	checkExists(filepath.Join(base, "parameter_sweep.csv"))
	checkExists(filepath.Join(base, "baseline_pipelines.json"))
	checkExists(filepath.Join(base, "kappa_window_table.csv"))
	checkExists(filepath.Join("scripts", "regtest_fee_profiles.ps1"))
	checkExists(filepath.Join("scripts", "signet_fee_profiles.ps1"))
	checkExists(filepath.Join("artifacts", "linked_acs_regtest.json"))
	checkExists(filepath.Join("artifacts", "linked_acs_signet.json"))

	var decisions attack.DecisionReport
	readJSON(filepath.Join(base, "attack_decisions.json"), &decisions)
	assert(decisions.Profile.CStarSat == decisions.Profile.VSat+decisions.Profile.VDepSat,
		"c_star must equal v+v_dep in attack_decisions.json")
	assert(hasDecision(decisions.Decisions, "Naive CRAB+He", true, -1),
		"Naive CRAB+He decision must be jointly profitable")
	assert(hasDecision(decisions.Decisions, "CRAB-He c*=v+v_dep", false, 0),
		"CRAB-He at c*=v+v_dep must be infeasible")

	var timeline experiments.AttackTimelineReport
	readJSON(filepath.Join(base, "attack_timeline.json"), &timeline)
	assert(timeline.Baseline.WidthSat > 0, "baseline attack_timeline width must be positive")
	assert(timeline.CRABHe.WidthSat == 0, "CRAB-He attack_timeline width must be zero")
	assert(timeline.Profile.CRABHeCStarSat == timeline.Profile.VSat+timeline.Profile.VDepSat,
		"attack_timeline c_star must equal v+v_dep")

	checkCSVNonEmpty(filepath.Join(base, "parameter_sweep.csv"))
	checkCSVNonEmpty(filepath.Join(base, "kappa_window_table.csv"))
	checkFeeProfile(filepath.Join("artifacts", "onchain", "regtest", "fee_profiles", "fee_profile_summary.csv"))
	checkFeeProfile(filepath.Join("artifacts", "onchain", "signet", "fee_profiles", "fee_profile_summary.csv"))
	checkCSVNonEmpty(filepath.Join("artifacts", "onchain", "regtest", "fee_profiles", "fee_profile_txids.csv"))
	checkCSVNonEmpty(filepath.Join("artifacts", "onchain", "signet", "fee_profiles", "fee_profile_txids.csv"))

	fmt.Println("artifact consistency checks passed")
}

func hasDecision(rows []attack.Decision, scheme string, jointlyProfitable bool, widthSat int64) bool {
	for _, r := range rows {
		if r.Scheme != scheme {
			continue
		}
		if r.JointlyProfitable != jointlyProfitable {
			return false
		}
		if widthSat >= 0 && r.WidthSat != widthSat {
			return false
		}
		return true
	}
	return false
}

func checkExists(path string) {
	if _, err := os.Stat(path); err != nil {
		fail("%s missing: %v", path, err)
	}
}

func readJSON(path string, dst any) {
	b, err := os.ReadFile(path)
	if err != nil {
		fail("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		fail("parse %s: %v", path, err)
	}
}

func checkCSVNonEmpty(path string) {
	f, err := os.Open(path)
	if err != nil {
		fail("open %s: %v", path, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		fail("parse %s: %v", path, err)
	}
	assert(len(rows) > 1, "%s must contain a header and at least one data row", path)
}

func checkFeeProfile(path string) {
	f, err := os.Open(path)
	if err != nil {
		fail("open %s: %v", path, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		fail("parse %s: %v", path, err)
	}
	assert(len(rows) == 16, "%s must contain 15 fee-profile runs plus header", path)
	for i, row := range rows[1:] {
		assert(len(row) >= 3, "%s row %d has too few columns", path, i+2)
		assert(row[2] == "True" || row[2] == "true", "%s row %d must be accepted", path, i+2)
	}
}

func assert(ok bool, format string, args ...any) {
	if !ok {
		fail(format, args...)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
