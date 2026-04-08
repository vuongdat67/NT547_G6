# Artifact Config (Option A)

This folder stores evidence for the Taproot single-UTXO redesign (CRAB-He Option A).
Legacy P2WSH and signet historical logs were removed.

## Regtest Node

Use Bitcoin Core in regtest server mode with fallback fee enabled:

```powershell
"C:\Program Files\Bitcoin\daemon\bitcoind.exe" -regtest -server "-fallbackfee=0.0002"
```

## Deploy Taproot Linked Leaf Spend

```powershell
go run ./scripts/deploy_linked_acs.go \
  -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" \
  -network regtest -wallet hehtlc_research \
  -fund-sat 3000000 -fee-sat 500000 -max-burn-btc 0.03
```

## Latest Regtest Evidence

- Fund TxID: 6aceae598d61ae2256508a4bdafc43568c77045f04f91fcca147c6423563038e
- Spend TxID: 5a64ec6a227bd6dc481e37e764d17712314933978bb7cdb9569adbdaff134245
- Artifact file: artifacts/linked_acs_regtest.json
