# CRAB-He Evaluation Results (Generated)

Generated at: 2026-04-03T15:56:23Z

Reference BTC price for fee conversion: $26,900/BTC (aligned with CRAB).

## 1) Transaction Table

| name_object | type | vbytes | fee_usd@2sat/vB | fee_usd@7sat/vB | fee_usd@10sat/vB | fee_usd@20sat/vB |
|---|---|---:|---:|---:|---:|---:|
| tx_fund | channel_tx | 338 | 0.18 | 0.64 | 0.91 | 1.82 |
| tx_commit_A (no HTLC) | channel_tx | 457 | 0.25 | 0.86 | 1.23 | 2.46 |
| tx_commit_A (HTLC+linked ACS) | channel_tx | 489 | 0.26 | 0.92 | 1.32 | 2.63 |
| tx_spend_A | channel_tx | 418 | 0.22 | 0.79 | 1.12 | 2.25 |
| tx_revoke_B | channel_tx | 192 | 0.10 | 0.36 | 0.52 | 1.03 |
| tx_revoke_ACS_std | channel_tx | 192 | 0.10 | 0.36 | 0.52 | 1.03 |
| tx_revoke_ACS_linked | channel_tx | 224 | 0.12 | 0.42 | 0.60 | 1.21 |
| tx_dep_A | htlc_tx | 190 | 0.10 | 0.36 | 0.51 | 1.02 |
| tx_dep_B | htlc_tx | 172 | 0.09 | 0.32 | 0.46 | 0.93 |
| tx_col_B | htlc_tx | 152 | 0.08 | 0.29 | 0.41 | 0.82 |
| tx_col_M | htlc_tx | 168 | 0.09 | 0.32 | 0.45 | 0.90 |

## 2) Linked ACS On-chain Evidence

| network | wallet | fund_txid | spend_txid | fund_vout | fund_value_sat | spend_value_sat | fee_sat | witness_order |
|---|---|---|---|---:|---:|---:|---:|---|
| regtest | test | b1bf01a8e60dd9864652460af0b17ec10d8a5c6246b052d77d108c12d9e9c40c | 9c793e227b99a167c30b845f3b4a09de761899c03ad79df0b2e96e4258f0983d | 0 | 10000 | 9000 | 1000 | <pre_b> <r^j_a> <redeemScript> |
| signet | test | 37bcd54fe81bfcb39a6212a6e9372cf03b024288d207f4e4fbd8df62102625d4 | bb425bdfac48e420e1e0711ed5def96a7fc98850609561a84518542a239b4b27 | 0 | 10000 | 8000 | 2000 | <pre_b> <r^j_a> <redeemScript> |

## 3) CLBA Summary

- crab_rational_width_sat: 2500000
- crab_byzantine_width_sat: 2500000
- crab_he_width_sat: 0
- crab_he_infeasible: true
- c_star_sat: 2500000

## 4) Coalition Summary

- fee_sat: 1000
- model_note: coalition threshold uses k*v_col (fee-independent SDRBA convention); feeSat retained for compatibility metadata
- k_max: 6
- single_miner_c_star_sat: 2500000
- single_miner_dominates: true

| k | bob_ub_sat | miner_lb_sat | width_sat | c_star_k_sat | feasible |
|---|---:|---:|---:|---:|---|
| 1 | 500000 | 500000 | 0 | 2500000 | false |
| 2 | 500000 | 1000000 | -500000 | 2000000 | false |
| 3 | 500000 | 1500000 | -1000000 | 1500000 | false |
| 4 | 500000 | 2000000 | -1500000 | 1000000 | false |
| 5 | 500000 | 2500000 | -2000000 | 500000 | false |
| 6 | 500000 | 3000000 | -2500000 | 0 | false |
| 7 | 500000 | 3500000 | -3000000 | 0 | false |
