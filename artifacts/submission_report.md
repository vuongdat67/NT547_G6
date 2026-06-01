# CRAB-He Submission Artifact Report

Generated: 2026-05-25T08:26:55Z

This report summarizes repository artifacts used by the CRAB-He paper. It is an evidence index, not a new theorem or a production-deployment claim.

## Attack Timeline Replay

| scheme | miner_lb_sat | bob_ub_sat | width_sat | selected_br_sat | profitable | outcome |
|---|---|---|---|---|---|---|
| Naive CRAB+He | 3000000 | 5500000 | 2500000 | 4250000 | true | profitable CLBA interval exists; stale-state and HTLC-side gains fund the bribe |
| CRAB-He | 3000000 | 3000000 | 0 | 0 | false | no jointly profitable BR exists; dep-B reveals pre_b and triggers fixed linked burn/fee spend |

## Parallel Independent Swaps

Invariant: `c*_n = v + n*v_dep` for independent standalone HTLC instances; this is not a routed-Lightning multi-hop claim.

| n | c_n_star_sat | overhead_sat |
|---|---|---|
| 1 | 2100000 | 100000 |
| 3 | 2300000 | 300000 |
| 5 | 2500000 | 500000 |
| 7 | 2700000 | 700000 |

## Linked ACS Evidence

| network | fundTxid | spendTxid | fundSat | burnSat | feeSat | createdAtUtc |
|---|---|---|---|---|---|---|
| regtest | 6aceae59...3563038e | 5a64ec6a...ff134245 | 3000000 | 2500000 | 500000 | 2026-04-08T12:25:56Z |
| signet | d77febbc...a598deaa | e9b82087...ee80074f | 3000000 | 2500000 | 500000 | 2026-04-08T13:55:45Z |

## Fee-Profile Campaign

| network | runs | successful |
|---|---|---|
| regtest | 15 | 15 |
| signet | 15 | 15 |

## Non-Claims

- No production miner bribery marketplace.
- No modified Bitcoin miner client performing real censorship.
- No confirmed CLBA mainnet incident.
- No full routed-Lightning HTLC model.
- No theorem-level multi-miner coalition game.
