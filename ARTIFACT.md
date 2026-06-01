# CRAB-He Artifact Map

This file maps the paper/proposal claims to the concrete repository evidence.

For a reviewer-facing Markdown summary generated from current artifacts, run:

```powershell
go run ./cmd/submission_report
```

Output:

- `artifacts/submission_report.md`

## Claim 1: Naive CRAB+He Has Positive CLBA Width

Evidence:

- `artifacts/experiments/attack_decisions.json`
- `artifacts/experiments/attack_timeline.json`
- `artifacts/experiments/parameter_sweep.csv`

Key invariant:

- `Naive CRAB+He`: `Miner-LB = 3,000,000`, `Bob-UB = 5,500,000`, `Width = 2,500,000`, `BR = 4,250,000`.

## Claim 2: Collateral-Only Inflation Fails

Evidence:

- `artifacts/experiments/attack_decisions.json`
- `artifacts/experiments/parameter_sweep.csv`

Key invariant:

- `Collateral-only c'=2c` keeps `Width = 2,500,000`; increasing `c` changes both Bob's upper bound and miner's lower bound equally.

## Claim 3: CRAB-He Closes The Bribery Interval

Evidence:

- `artifacts/experiments/attack_decisions.json`
- `artifacts/experiments/attack_timeline.json`
- `go run ./cmd/verify_artifacts`

Key invariant:

- `c* = v + v_dep = 3,000,000`.
- At `c*`, `Miner-LB = Bob-UB = 3,000,000`, so no jointly profitable `BR` exists.

## Claim 3b: Parallel Independent Swaps Scale Linearly

Evidence:

- `artifacts/experiments/parallel_swaps_table.csv`
- `artifacts/experiments/multi_hop_table.csv` (legacy alias only)
- `artifacts/publication/table_parallel_swaps.tex`

Key invariant:

- For `n` independent standalone HTLC instances, `c*_n = v + n*v_dep`.
- This is not a routed-Lightning multi-hop claim.

## Claim 4: Linked ACS Is Script-Feasible

Evidence:

- `scripts/deploy_linked_acs.go`
- `artifacts/linked_acs_regtest.json`
- `artifacts/linked_acs_signet.json`
- `artifacts/crab_he_results.json`

Confirmed analytical profile:

- `fund = 3,000,000 sat`
- `burn = 2,500,000 sat`
- `fee = 500,000 sat`
- `commit = 281 vB`
- `spend = 246 vB`

## Claim 5: Fee-Profile Campaign Is Present

Evidence:

- `scripts/regtest_fee_profiles.ps1`
- `scripts/signet_fee_profiles.ps1`
- `artifacts/onchain/regtest/fee_profiles/fee_profile_summary.csv`
- `artifacts/onchain/signet/fee_profiles/fee_profile_summary.csv`
- `artifacts/onchain/regtest/fee_profiles/fee_profile_txids.csv`
- `artifacts/onchain/signet/fee_profiles/fee_profile_txids.csv`

Key invariant:

- 5 fee levels x 3 runs x 2 networks = 30 accepted runs.

## What This Code Proves

The code proves:

- payoff/SDRBA-style accept-reject decisions for a side deal,
- deterministic CLBA replay under those payoff inequalities,
- linked Taproot ACS script feasibility on regtest and signet,
- local artifact consistency through `go run ./cmd/verify_artifacts`.

## Non-Claims

The artifact does not claim:

- a production miner bribery marketplace,
- a modified Bitcoin miner client that performs real censorship,
- a confirmed CLBA mainnet incident,
- a full routed-Lightning HTLC model,
- a theorem-level multi-miner coalition game.
