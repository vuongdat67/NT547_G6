package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/crab-he/internal/channel"
)

func main() {
	fmt.Println("=== CRAB-He Coalition Extension Evaluation ===")
	fmt.Println()

	v := big.NewInt(2_000_000)
	vDep := big.NewInt(1_000_000)
	vCol := big.NewInt(500_000)
	delta := big.NewInt(1_000)
	feeSat := int64(1_000)

	params, err := channel.NewParams(v, vDep, vCol, delta, 144, 288, 6, 3)
	if err != nil {
		panic(err)
	}
	fmt.Println("Model note: coalition threshold uses k*v_col (fee-independent SDRBA convention).")
	fmt.Println("feeSat is retained for compatibility metadata in command/report APIs.")
	fmt.Println()

	fmt.Printf("Base parameters: v=%d, v_dep=%d, v_col=%d, c*=%d\n\n",
		params.V.Int64(), params.VDep.Int64(), params.VCol.Int64(), params.CStar.Int64())

	fmt.Println("=== Table: Coalition Size vs. CLBA Feasibility (current c*) ===")
	fmt.Printf("%-6s %-18s %-18s %-18s %-12s\n", "k", "Bob-UB (sat)", "Miner-LB_k (sat)", "Width_k (sat)", "Feasible?")
	fmt.Println(strings.Repeat("-", 76))

	kMax := channel.KMax(params, feeSat)
	for k := 1; k <= min(10, kMax+1); k++ {
		lambdaK := float64(k) * 0.05
		if lambdaK >= 0.5 {
			lambdaK = 0.49
		}
		ca, err := channel.NewCoalitionAnalysis(params, k, lambdaK, feeSat)
		if err != nil {
			fmt.Printf("k=%d: error %v\n", k, err)
			continue
		}
		feasible := "YES"
		if !ca.IsCLBAFeasibleCoalition() {
			feasible = "NO"
		}
		fmt.Printf("%-6d %-18s %-18s %-18s %-12s\n",
			k,
			ca.BobUBLinked().String(),
			ca.MinerLBCoalition().String(),
			ca.WidthCoalition().String(),
			feasible,
		)
	}
	fmt.Printf("\nk_max (natural infeasibility threshold) = %d\n\n", kMax)

	fmt.Println("=== Table: Required c* to Block Coalition of Size k ===")
	fmt.Printf("%-6s %-22s %-22s %-18s\n", "k", "c*_k needed (sat)", "vs single-miner c*", "Overhead change")
	fmt.Println(strings.Repeat("-", 72))

	cStarSingle := params.CStar.Int64()
	for k := 1; k <= 7; k++ {
		ca, err := channel.NewCoalitionAnalysis(params, k, 0.05, feeSat)
		if err != nil {
			panic(err)
		}
		cStarK := ca.CStarForCoalition().Int64()
		diff := cStarK - cStarSingle
		sign := "+"
		if diff < 0 {
			sign = ""
		}
		fmt.Printf("%-6d %-22d %-22d %s%d sat\n",
			k, cStarK, cStarSingle, sign, diff)
	}

	fmt.Println()
	fmt.Println("Key insight: c*_k DECREASES as k increases.")
	fmt.Println("Larger coalition requires LESS collateral to block, not more.")
	fmt.Println("Current single-miner c* already dominates all coalition sizes.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
