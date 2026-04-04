# CRAB-He: Implementation

Proof-of-concept implementation of CRAB-He protocol.

## Structure

```
crab-he/
├── cmd/
│   └── main.go
├── internal/
│   ├── channel/
│   │   ├── params.go
│   │   └── transactions.go
│   └── htlc/
│       └── htlc.go
├── go.mod
└── README.md
```

## Run

```bash
go run ./cmd/main.go
```

## Regtest TxID Smoke Test

If Bitcoin Core regtest is running and a wallet is loaded (default wallet name: `test`), run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\regtest_smoke.ps1
```

If your node reports fee estimation issues on regtest, pass an explicit fee rate:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\regtest_smoke.ps1 -FeeRateSatVb 1
```

The script sends a real regtest transaction, mines 1 block, and writes tx evidence to:

`artifacts/regtest_txids.json`

## Signet TxID Smoke Test

If your signet node is running and wallet has funds, run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\signet_smoke.ps1 -WalletName test -AmountBtc 0.0002
```

Optional: wait for 1 confirmation (or customize poll/wait):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\signet_smoke.ps1 -WalletName test -AmountBtc 0.0002 -TargetConfirmations 1 -PollSeconds 30 -MaxWaitMinutes 60
```

If wallet is not loaded yet, include:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\signet_smoke.ps1 -WalletName test -TryLoadWallet
```

The signet artifact is written to:

`artifacts/signet_txids.json`

## Linked ACS Deploy Script (Actual Script Spend)

This script creates a linked ACS P2WSH output and spends it using witness
`<pre_b> <r^j_a> <redeemScript>` with corrected stack order.

Run on regtest (auto-mines fund/spend blocks):

```powershell
go run ./scripts/deploy_linked_acs.go -network regtest -wallet test
```

If wallet exists but is not loaded, the script auto-loads it by default.
If wallet may not exist yet, add:

```powershell
go run ./scripts/deploy_linked_acs.go -network regtest -wallet test -create-wallet-if-missing
```

Run on signet (no auto-mining):

```powershell
go run ./scripts/deploy_linked_acs.go -network signet -wallet hehtlc_research -fund-sat 10000 -fee-sat 1000
```

Artifact output:

`artifacts/linked_acs_regtest.json` or `artifacts/linked_acs_signet.json`

## Coalition Evaluation

To evaluate the coalition extension from `coalition_dev_prompt.md`, run:

```powershell
go run ./cmd/coalition_eval/main.go
```

This prints coalition-size feasibility and required collateral tables derived from the current CRAB-He parameters.

## Experiment Guide Runner (Checklist Coverage)

To execute parameter-grid sweeps, seed-based simulation, baseline adapters, and runtime telemetry from
`Dung/experiment_guide.md`, run:

```powershell
go run ./cmd/experiment_runner
```

This generates:

- `artifacts/experiments/experiment_summary.json`
- `artifacts/experiments/parameter_sweep.csv`
- `artifacts/experiments/multi_hop_table.csv` (includes n=1,3,5,7)
- `artifacts/experiments/baseline_pipelines.json` (transaction-level MAD-HTLC and He-HTLC standalone paths)
- `artifacts/experiments/seed_simulation_summary.json` (30-seed synthetic Monte Carlo over analytical widths; paired t-test + Wilcoxon; includes `heConditionValid` flags)

The runner includes explicit baseline adapters in code for:

- MAD-HTLC standalone reference
- He-HTLC standalone condition margin
- CRAB collateral-only baseline
- CRAB-He linked-revocation threshold cases (c* - eps, c*, c* + eps)

## Repeated On-Chain Grid and Seed Orchestrator

To run repeated on-chain executions across the parameter grid and seed set:

```powershell
go run ./cmd/onchain_orchestrator -dry-run=false -seed-runs 30 -networks regtest,signet -wallet test
```

For signet campaigns, public mempool policy (for example `too-long-mempool-chain`) may reject
bursty runs. Use retry and pacing flags when needed:

```powershell
go run ./cmd/onchain_orchestrator -dry-run=false -seed-runs 1 -networks signet -wallet test -retry-attempts 3 -retry-delay-ms 15000
```

Quick planning sample (no broadcast):

```powershell
go run ./cmd/onchain_orchestrator -dry-run -max-configs 2 -seed-runs 2
```

Outputs:

- `artifacts/onchain/repeated_onchain_summary.json`
- `artifacts/onchain/repeated_onchain_runs.csv`
- per-run deployment artifacts under `artifacts/onchain/<network>/<config>/seed_xxx.json`

## Publication-Ready Tables and Plots

To generate paper-ready LaTeX tables and SVG figures directly from artifacts:

```powershell
go run ./cmd/publish_results
```

Outputs:

- `artifacts/publication/table_main_results.tex`
- `artifacts/publication/table_multi_hop.tex`
- `artifacts/publication/table_seed_stats.tex`
- `artifacts/publication/table_onchain_runs.tex`
- `artifacts/publication/fig_multi_hop_cnstar.svg`
- `artifacts/publication/fig_baseline_success.svg`
- `artifacts/publication/fig_onchain_success.svg`

``` go 
go build ./... 
go vet ./...
go build ./scripts/deploy_linked_acs.go
go run ./scripts/deploy_linked_acs.go -h
```