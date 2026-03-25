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

