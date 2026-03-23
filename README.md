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

