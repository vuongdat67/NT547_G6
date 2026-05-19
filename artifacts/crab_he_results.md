# CRAB-He Evaluation Results (Generated)

Generated at: 2026-05-18T04:12:33Z

Reference BTC price for fee conversion: $26,900/BTC (aligned with CRAB).

## 1) Transaction Table

| name_object | type | vbytes | fee_usd@2sat/vB | fee_usd@7sat/vB | fee_usd@10sat/vB | fee_usd@20sat/vB |
|---|---|---:|---:|---:|---:|---:|
| tx_fund | channel_tx | 338 | 0.18 | 0.64 | 0.91 | 1.82 |
| tx_commit_A (no HTLC) | channel_tx | 283 | 0.15 | 0.53 | 0.76 | 1.52 |
| tx_commit_A (HTLC+linked ACS) | channel_tx | 281 | 0.15 | 0.53 | 0.76 | 1.51 |
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
| tx_commit_A (HTLC+linked ACS) | 225 | 224 | 449 | 1124 | 281 | 1 | 3 | 4 |

## 2) Linked ACS On-chain Evidence

| network | wallet | fund_txid | spend_txid | fund_vout | fund_value_sat | spend_value_sat | fee_sat | witness_order |
|---|---|---|---|---:|---:|---:|---:|---|
| regtest | hehtlc_research | 6aceae598d61ae2256508a4bdafc43568c77045f04f91fcca147c6423563038e | 5a64ec6a227bd6dc481e37e764d17712314933978bb7cdb9569adbdaff134245 | 0 | 3000000 | 2500000 | 500000 | <sig_B> <sig_A> <pre_b> <r^j_a> <linkedLeafScript> <controlBlock> |
| signet | hehtlc_research | d77febbc5f3778d089955541bfa881d86d1db7ec0720a5ef6d71c0eaa598deaa | e9b820874ff5aa2f3b5da2e0f8a8283d2091482731fa4a7279b8703aee80074f | 0 | 3000000 | 2500000 | 500000 | <sig_B> <sig_A> <pre_b> <r^j_a> <linkedLeafScript> <controlBlock> |

## 3) CLBA Summary

- crab_rational_width_sat: 2500000
- crab_byzantine_width_sat: 2500000
- crab_he_width_sat: 0
- crab_he_infeasible: true
- c_star_sat: 3000000

## 4) Coalition Summary

> **DIAGNOSTIC ONLY** — values in this section are interpretive support under the He-HTLC SDRBA standalone assumptions and are NOT claimed as composed-model theorems. Do not cite these rows as security proofs for CRAB-He coalition resistance; see Lemma (Coalition censorship probability) in the paper for the theorem-level statement.

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
