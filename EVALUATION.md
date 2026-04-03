# CRAB-He Evaluation Workflow

This document explains how to reproduce and validate CRAB-He-native data used in Section 7 tables.

## 1) Source-of-truth inputs

- CRAB-He implementation outputs: ./cmd/main.go and ./cmd/eval_report/main.go.
- Linked ACS deployment artifacts: ./artifacts/linked_acs_regtest.json and ./artifacts/linked_acs_signet.json.

## 2) Generate canonical CRAB-He result tables

From the crab-he folder:

go run ./cmd/eval_report

This writes:
- ./artifacts/crab_he_results.json
- ./artifacts/crab_he_results.md

The generator includes:
- tx names and vbytes used in CRAB-He table
- fee estimates (2/7/10/20 sat/vB)
- collateral/CLBA width summary
- linked ACS on-chain evidence (if artifact files exist)
- fee conversion reference price: $26,900/BTC (consistent with CRAB)

## 3) Run linked ACS deployment evidence

Regtest:

go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network regtest -wallet test -fund-sat 10000 -fee-sat 1000 -auto-mine-regtest

Signet:

go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network signet -wallet test -fund-sat 10000 -fee-sat 2000 -max-wait-seconds 600

Artifacts written by the script:
- ./artifacts/linked_acs_regtest.json
- ./artifacts/linked_acs_signet.json

## 4) Canonical extracted tables

Machine-readable table data for the manuscript is stored in:
- ./artifacts/crab_he_results.json

## 5) Manuscript mapping

Paper table rows in ../crab-hehtlc.md Section 7 are aligned to `crab_he_results.json` and linked ACS deployment artifacts.

## 6) Consistency checks

- Linked ACS witness order in code and manuscript must be:
  <pre_b> <r^j_a>
- He-HTLC script family remains HASH160-based.
- Linked ACS remains SHA256-based.

## 7) Data scope policy

- Primary quantitative claims use only CRAB-He-generated data and artifacts from this repository.
- Legacy data under ../bin is considered historical reference only and is not used as primary Section 7 evidence.

## 8) Experiment-guide automation (grid, seeds, statistics)

Run the experiment guide pipeline:

```powershell
go run ./cmd/experiment_runner
```

Outputs:

- `./artifacts/experiments/experiment_summary.json`
  - full parameter grid (`v_dep/v`, `v_col/v_dep`, `kappa`, `n`)
  - CRAB collateral-only invariance checks
  - CRAB-He threshold checks at `c* - eps`, `c*`, `c* + eps`
  - baseline adapters (MAD-HTLC standalone, He-HTLC standalone, CRAB, CRAB-He)
  - runtime telemetry (block count from artifacts, witness generation time, script validation time)
- `./artifacts/experiments/parameter_sweep.csv`
- `./artifacts/experiments/multi_hop_table.csv` (explicit rows for `n=1,3,5,7`)
- `./artifacts/experiments/seed_simulation_summary.json`
  - 30-seed paired simulation per configuration
  - paired t-test and Wilcoxon signed-rank summaries

These artifacts are intended to cover the remaining checklist items from
`check-list.md` and `Dung/experiment_guide.md`.
