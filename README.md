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

This script creates a Taproot single-UTXO linked ACS output on `out[2]` with two leaves:

- leaf-CRAB: `OP_SHA256 H(r^j_a) OP_EQUAL`
- leaf-linked: `H(r^j_a)` + `H(pre_b)` + 2-of-2 Schnorr `OP_CHECKSIGADD`

The linked spend is executed via script-path witness:
`<sig_B> <sig_A> <pre_b> <r^j_a> <linkedLeafScript> <controlBlock>`.

Run on regtest (auto-mines fund/spend blocks):

```powershell
go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network regtest -wallet hehtlc_research -fund-sat 3000000 -fee-sat 500000 -max-burn-btc 0.03
```

If wallet exists but is not loaded, the script auto-loads it by default.
If wallet may not exist yet, add:

```powershell
go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network regtest -wallet hehtlc_research -create-wallet-if-missing
```

Run on signet (no auto-mining):

```powershell
go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network signet -wallet hehtlc_research -fund-sat 2500000 -fee-sat 500000 -max-burn-btc 0.03 -max-wait-seconds 600
```

The tested signet profile above requires a funded wallet with at least `fund-sat`
available as trusted balance.

Artifact output:

`artifacts/linked_acs_regtest.json` or `artifacts/linked_acs_signet.json`

Latest verified evidence (2026-04-08):

- regtest fund: `6aceae598d61ae2256508a4bdafc43568c77045f04f91fcca147c6423563038e`
- regtest spend: `5a64ec6a227bd6dc481e37e764d17712314933978bb7cdb9569adbdaff134245`
- signet fund: `d77febbc5f3778d089955541bfa881d86d1db7ec0720a5ef6d71c0eaa598deaa`
- signet spend: `e9b820874ff5aa2f3b5da2e0f8a8283d2091482731fa4a7279b8703aee80074f`

## Coalition Evaluation

To evaluate the coalition extension from `coalition_dev_prompt.md`, run:

```powershell
go run ./cmd/coalition_eval/main.go
```

This prints coalition-size feasibility and required collateral tables derived from the current CRAB-He parameters.

## Experiment Guide Runner (Checklist Coverage)

To execute parameter-grid sweeps, baseline adapters, and runtime telemetry from
`Dung/experiment_guide.md`, run:

```powershell
go run ./cmd/experiment_runner
```

This generates:

- `artifacts/experiments/experiment_summary.json`
- `artifacts/experiments/parameter_sweep.csv`
- `artifacts/experiments/multi_hop_table.csv` (includes n=1,3,5,7)
- `artifacts/experiments/baseline_pipelines.json` (transaction-level MAD-HTLC and He-HTLC standalone paths)

Note: `seed_simulation_summary.json` is intentionally removed by the runner in
the current analytical-only pipeline.

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

- `artifacts/publication/table_multi_hop.tex`
- `artifacts/publication/fig_multi_hop_cnstar.svg`
- `artifacts/publication/publication_manifest.json`

Note: obsolete publication artifacts (`table_main_results.tex`, `table_seed_stats.tex`,
`table_onchain_runs.tex`, `fig_baseline_success.*`, `fig_onchain_success.*`) are removed
by `cmd/publish_results` to keep paper assets consistent with the current manuscript scope.

## One-Command Publication + Paper Sync

To regenerate evaluation/experiment/publication artifacts and sync publication tables/images
into paper folders (`../tex` and `../Dung/paper` by default), run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\publish_sync.ps1
```

This script also mirrors linked deployment evidence to:

- `artifacts/onchain/regtest/latest_linked_acs.json`
- `artifacts/onchain/signet/latest_linked_acs.json`

Optional flags:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\publish_sync.ps1 -SkipExperimentRunner
powershell -ExecutionPolicy Bypass -File .\scripts\publish_sync.ps1 -PaperDirs ..\tex
powershell -ExecutionPolicy Bypass -File .\scripts\publish_sync.ps1 -SvgConverter browser
powershell -ExecutionPolicy Bypass -File .\scripts\publish_sync.ps1 -SvgConverter texlive
```

`-SvgConverter` modes:

- `auto` (default): native tools (`inkscape`/`magick`/`rsvg-convert`) then browser fallback
- `native`: only native converter tools
- `browser`: force Chrome/Edge headless conversion
- `texlive`: force TeX Live (`lualatex` + `svg` package, requires shell-escape workflow)
- `none`: skip SVG→PDF conversion (only copy SVG)

If `-SvgConverter texlive` cannot complete SVG conversion (for example missing Inkscape bridge), the script automatically falls back to Chrome/Edge headless when available.

``` go 
go build ./... 
go vet ./...
go build ./scripts/deploy_linked_acs.go
go run ./scripts/deploy_linked_acs.go -h
```