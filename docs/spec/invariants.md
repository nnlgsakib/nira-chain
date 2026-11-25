# Invariant registry

| ID | Component | Statement (short) | Checked by |
|---|---|---|---|
| INV-STAKE-CAP-1 | STAKE | accepted delegation leaves validator ≤ cap (when active) | x/policy/keeper/power_cap_test.go |
| INV-STAKE-CAP-2 | STAKE | undelegation never blocked by the cap | power_cap_test.go, tests/cosmos-e2e, tests/evm-e2e |
| INV-STAKE-CAP-3 | STAKE | no cap scratch entry survives a staking call | power_cap_test.go |
| INV-COMM-1 … 10 | COMM | see docs/spec/community.md §9 | x/community tests (to be written) |
