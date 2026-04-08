# CRAB-He Evaluation Results (Generated)

Generated at: 2026-04-08T00:03:08Z

Reference BTC price for fee conversion: $26,900/BTC (aligned with CRAB).

## 1) Transaction Table

| name_object | type | vbytes | fee_usd@2sat/vB | fee_usd@7sat/vB | fee_usd@10sat/vB | fee_usd@20sat/vB |
|---|---|---:|---:|---:|---:|---:|
| tx_fund | channel_tx | 338 | 0.18 | 0.64 | 0.91 | 1.82 |
| tx_commit_A (no HTLC) | channel_tx | 283 | 0.15 | 0.53 | 0.76 | 1.52 |
| tx_commit_A (HTLC+linked ACS) | channel_tx | 433 | 0.23 | 0.82 | 1.16 | 2.33 |
| tx_spend_A | channel_tx | 418 | 0.22 | 0.79 | 1.12 | 2.25 |
| tx_revoke_B | channel_tx | 192 | 0.10 | 0.36 | 0.52 | 1.03 |
| tx_revoke_ACS_std | channel_tx | 192 | 0.10 | 0.36 | 0.52 | 1.03 |
| tx_revoke_ACS_linked | channel_tx | 246 | 0.13 | 0.46 | 0.66 | 1.32 |
| tx_dep_A | htlc_tx | 190 | 0.10 | 0.36 | 0.51 | 1.02 |
| tx_dep_B | htlc_tx | 172 | 0.09 | 0.32 | 0.46 | 0.93 |
| tx_col_B | htlc_tx | 152 | 0.08 | 0.29 | 0.41 | 0.82 |
| tx_col_M | htlc_tx | 168 | 0.09 | 0.32 | 0.45 | 0.90 |

## 1.1) Serialized Size Evidence (commit templates)

Formula: vbytes = ceil(weight / 4), weight = base*3 + total, witness = total - base.

| name | base_bytes | witness_bytes | total_bytes | weight | vbytes | inputs | outputs | witness_items |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| tx_commit_A (no HTLC) | 227 | 224 | 451 | 1132 | 283 | 1 | 3 | 4 |
| tx_commit_A (HTLC+linked ACS) | 377 | 224 | 601 | 1732 | 433 | 1 | 4 | 4 |

## 2) Linked ACS On-chain Evidence

| network | wallet | fund_txid | spend_txid | fund_vout | fund_value_sat | spend_value_sat | fee_sat | witness_order |
|---|---|---|---|---:|---:|---:|---:|---|
| regtest | hehtlc_research | 7802fbdda123a7f6bfc4f5f201112e527b62fa58080ea5077067e8fd082593e2 | 180fe4abf1e06e9c1d529da463d987c07c6dcbad39f47ed2063a3600a6a247fa | 1 | 10000 | 5000 | 5000 | <dummy> <sig_A> <sig_B> <pre_b> <r^j_a> <redeemScript> |
| signet | hehtlc_research | d393b63a9f901138ae55557200c6496f8ff3fbe4ab1335a2198e7ebff8c2d17a | 44927340565152697cfae902878510cc7c8849caa6c718fa1eed565f0f23acbb | 0 | 10000 | 5000 | 5000 | <dummy> <sig_A> <sig_B> <pre_b> <r^j_a> <redeemScript> |

## 3) CLBA Summary

- crab_rational_width_sat: 2500000
- crab_byzantine_width_sat: 2500000
- crab_he_width_sat: 0
- crab_he_infeasible: true
- c_star_sat: 3000000

## 4) Coalition Summary

- fee_sat: 1000
- model_note: derived comparison under He-HTLC SDRBA assumptions (k*v_col threshold); this section is interpretive support, not an independent theorem claim
- k_max: 6
- single_miner_c_star_sat: 3000000
- single_miner_dominates: true

| k | bob_ub_sat | miner_lb_sat | width_sat | c_star_k_sat | feasible |
|---|---:|---:|---:|---:|---|
| 1 | 3000000 | 500000 | 2500000 | 3000000 | true |
| 2 | 3000000 | 1000000 | 2000000 | 2500000 | true |
| 3 | 3000000 | 1500000 | 1500000 | 2000000 | true |
| 4 | 3000000 | 2000000 | 1000000 | 1500000 | true |
| 5 | 3000000 | 2500000 | 500000 | 1000000 | true |
| 6 | 3000000 | 3000000 | 0 | 500000 | false |
| 7 | 3000000 | 3500000 | -500000 | 0 | false |
