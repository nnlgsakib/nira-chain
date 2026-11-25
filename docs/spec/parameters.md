# Parameter registry

All values are (proposed) unless stated. Governance changes every parameter through the owning module's MsgUpdateParams.

| ID | Component | Default | Bounds | Where |
|---|---|---|---|---|
| P-STAKE-max_validators | STAKE | 100 | 4–150 | x/staking params |
| P-STAKE-unbonding_time | STAKE | 21 d | 7–28 d (> evidence age) | x/staking params |
| P-STAKE-min_commission_rate | STAKE | 5 % | 0–20 % | x/staking params |
| P-STAKE-slash_double_sign | STAKE | 5 % | 1–100 % | x/slashing params |
| P-STAKE-slash_downtime | STAKE | 0.01 % | 0–1 % | x/slashing params |
| P-STAKE-signed_blocks_window | STAKE | 10 000 | ≥ 1 000 | x/slashing params |
| P-STAKE-min_signed_per_window | STAKE | 5 % | 1–90 % | x/slashing params |
| P-STAKE-max_validator_power_bps | STAKE | 1000 | 300–3333 | x/nirachain params |
| P-STAKE-cap_activation_bonded | STAKE | 5 000 000 NIRA | ≥ 0 | x/nirachain params |
| P-STAKE-cap_activation_min_validators | STAKE | 15 | ceil(10000/bps)–1000 | x/nirachain params |
| P-EXEC-block_max_gas | EXEC | 100 000 000 | finite | consensus params |
| P-EXEC-evm_chain_id | EXEC | 47201 | fixed forever | app constant (Q2) |
| P-GOV-min_deposit | GOV | 10 000 NIRA | — | x/gov params |
| P-COMM-* | COMM | see docs/spec/community.md §7 | | x/community params |
