# CRAB-He: Artifact and Proof-of-Concept

This repository contains the CRAB-He proof-of-concept used by the paper:

> CRAB-He: Securing Lightning Payment Channels Against Actively Rational Miners in Composed HTLC Settings

The code supports the paper's artifact-backed claims: CLBA payoff-width analysis,
collateral-only impossibility checks, deterministic attack replay, Taproot linked
ACS script feasibility on Bitcoin regtest/signet, and publication table/figure
generation.

The artifact is intentionally scoped. It does not implement a production miner
bribery marketplace, modify Bitcoin miner policy, claim a confirmed mainnet
incident, or model full routed-Lightning HTLCs.

## Structure

```
crab-he/
├── cmd/
│   ├── main.go
│   ├── coalition_eval/
│   ├── eval_report/
│   ├── experiment_runner/
│   ├── onchain_orchestrator/
│   ├── publish_results/
│   ├── submission_report/
│   └── verify_artifacts/
├── internal/
│   ├── attack/
│   ├── channel/
│   ├── experiments/
│   └── htlc/
├── scripts/
│   ├── deploy_linked_acs.go
│   ├── regtest_fee_profiles.ps1
│   ├── signet_fee_profiles.ps1
│   └── publish_sync.ps1
├── artifacts/
├── go.mod
└── README.md
```

## Quick Verification

Run the local proof/artifact checks:

```powershell
go test ./...
go run ./cmd/verify_artifacts
```

Generate a reviewer-friendly artifact summary:

```powershell
go run ./cmd/submission_report
```

This writes:

`artifacts/submission_report.md`

For a local end-to-end verification pass:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\ci_local.ps1
```

## Demo Command

```powershell
go run ./cmd/main.go
```

This prints the representative CRAB-He transaction model and threshold values.

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
go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network signet -wallet hehtlc_research -fund-sat 3000000 -fee-sat 500000 -max-burn-btc 0.03 -max-wait-seconds 600
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

To inspect the diagnostic coalition-size calculations, run:

```powershell
go run ./cmd/coalition_eval/main.go
```

This prints coalition-size feasibility and required collateral tables derived
from the current CRAB-He parameters. These outputs are diagnostic support only;
the paper does not claim a theorem-level multi-miner coalition game.

## Experiment Guide Runner (Checklist Coverage)

To execute parameter-grid sweeps, baseline adapters, and runtime telemetry from
the current experiment guide, run:

```powershell
go run ./cmd/experiment_runner
```

This generates:

- `artifacts/experiments/experiment_summary.json`
- `artifacts/experiments/parameter_sweep.csv`
- `artifacts/experiments/attack_decisions.json` (SDRBA-style payoff mock: Bob offer, miner accept/reject)
- `artifacts/experiments/attack_timeline.json`
- `artifacts/experiments/attack_timeline.csv`
- `artifacts/experiments/parallel_swaps_table.csv` (includes n=1,3,5,7)
- `artifacts/experiments/multi_hop_table.csv` (legacy alias; semantically this is parallel-swap data)
- `artifacts/experiments/kappa_window_table.csv`
- `artifacts/experiments/baseline_pipelines.json` (transaction-level MAD-HTLC and He-HTLC standalone paths)

Note: legacy `seed_simulation_summary.json` is no longer generated by the
current analytical-only pipeline.

The runner includes explicit baseline adapters in code for:

- MAD-HTLC standalone reference
- He-HTLC standalone condition margin
- CRAB collateral-only baseline
- CRAB-He linked-revocation threshold cases (c* - eps, c*, c* + eps)
- SDRBA-style payoff decisions for Bob/miner side-deal acceptance
- deterministic CLBA attack-timeline replay for naive CRAB+He vs CRAB-He

To verify artifact consistency after generation:

```powershell
go run ./cmd/verify_artifacts
```

The checker validates the key paper invariants: `c* = v+v_dep`, baseline CLBA has a positive interval, CRAB-He has width zero at `c*`, and the required CSV/JSON artifacts are present.

For a local end-to-end verification pass:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\ci_local.ps1
```

See `ARTIFACT.md` for the claim-to-evidence map used by the paper/proposal.

## What This Code Proves / Does Not Prove

Proves:

- SDRBA-style payoff decision: Bob offers `BR`, miner accepts/rejects by utility.
- Deterministic CLBA replay: baseline has a positive interval; CRAB-He closes it at `c*`.
- Linked Taproot ACS script feasibility on regtest and signet.
- Artifact consistency via `go run ./cmd/verify_artifacts`.

Does not prove or claim:

- a production miner bribery marketplace,
- a modified Bitcoin miner client that performs real censorship,
- a confirmed CLBA mainnet incident,
- a full routed-Lightning HTLC model,
- a theorem-level multi-miner coalition game.

## Repeated On-Chain Grid and Seed Orchestrator

To run repeated on-chain executions across the parameter grid and seed set
(operational command template; published paper evidence may use a smaller confirmed subset):

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

- `artifacts/publication/table_parallel_swaps.tex`
- `artifacts/publication/fig_parallel_swaps_cnstar.svg`
- `artifacts/publication/table_multi_hop.tex` (legacy alias)
- `artifacts/publication/fig_multi_hop_cnstar.svg` (legacy alias)
- `artifacts/publication/publication_manifest.json`

To generate a Markdown submission summary from the same artifacts:

```powershell
go run ./cmd/submission_report
```

Note: obsolete publication artifacts (`table_main_results.tex`, `table_seed_stats.tex`,
`table_onchain_runs.tex`, `fig_baseline_success.*`, `fig_onchain_success.*`) are removed
by `cmd/publish_results` to keep paper assets consistent with the current manuscript scope.

## One-Command Publication + Paper Sync

To regenerate evaluation/experiment/publication artifacts and sync publication tables/images
into paper folders (`../tex` and `../tex/13` by default), run:

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

## Developer Checks

```powershell
go build ./...
go vet ./...
go build ./scripts/deploy_linked_acs.go
go run ./scripts/deploy_linked_acs.go -h
```
