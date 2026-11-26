# Invariant registry

Every invariant names the test that checks it. Tests reference the ID in their name or comment (spec ↔ code traceability).

| ID | Component | Statement (short) | Checked by |
|---|---|---|---|
| INV-STAKE-CAP-1 | STAKE | an accepted delegation leaves the validator ≤ cap (when the cap is active) | `x/policy/keeper/power_cap_test.go`: TestINV_STAKE_CAP_1_*, TestINV_STAKE_CAP_Randomised |
| INV-STAKE-CAP-2 | STAKE | undelegation is never blocked by the cap | TestINV_STAKE_CAP_2_*; `tests/cosmos-e2e/e2e.sh` #5; `tests/evm-e2e/e2e.mjs` precompile undelegate |
| INV-STAKE-CAP-3 | STAKE | no cap scratch entry survives a staking call | TestINV_STAKE_CAP_3_NoResidue |
| INV-STAKE-SELF-1 | STAKE | every validator was created with min_self_delegation ≥ min_self_bond (SDK keeps self-bond ≥ it or jails) | TestINV_STAKE_SELF_1_MinSelfBond; `tests/cosmos-e2e/e2e.sh` #8; `tests/evm-e2e/e2e.mjs` createValidator |
| INV-COMM-1 | COMM | paid ≤ budget; paid + community pool = budget | `x/community/keeper/community_test.go`: TestProofAndSettlement |
| INV-COMM-2 | COMM | every paid unit has a TaskDone record from a proof that passed C1–C12 | TestProofAndSettlement, TestSubmitTaskProofRejections |
| INV-COMM-3 | COMM | proofs only from nodes registered before the epoch, with onion base ≤ epoch | TestSubmitTaskProofRejections (C2), TestOfflineNodeNoPenaltyAndRejoin |
| INV-COMM-4 | COMM | module balance = active + unbonding bonds + open deposits + budget | `checkInvariants` (every test, every block in TestINV_COMM_Randomised) |
| INV-COMM-5 | COMM | TotalActiveBond = Σ active bonds | `checkInvariants`; TestGenesisRoundTrip |
| INV-COMM-6 | COMM | proofs per epoch ≤ cap; units per node ≤ tasks_per_selection | `checkInvariants`; TestSubmitTaskProofRejections (C7, C8) |
| INV-COMM-7 | COMM | app-hash ring bounded (window + epoch_blocks); proofs only against recorded hashes | `checkInvariants`; TestSubmitTaskProofRejections (C9) |
| INV-COMM-8 | COMM | hooks never return errors or emit validator updates | TestHookFailureIsolated |
| INV-COMM-9 | COMM | onion anchor strictly advances; H^d(new) = old | `x/community/types/derive_test.go`: TestINV_COMM_9_Reveal; TestRevealIsGasMetered |
| INV-COMM-10 | COMM | open alerts ≤ max; ≤ 1 alert per (height, reporter) | `checkInvariants`; TestDivergenceAlerts |
