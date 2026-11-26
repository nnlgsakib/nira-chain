# Spec — GOV: on-chain governance

- Version 0.1 (2026-11-25). Mechanism skill: protocol-governance (spec written without loading it; aligned with the threat model TM-19). Surface: Cosmos SDK x/gov v1, x/upgrade, x/circuit; EVM gov precompile `0x…0805`.

## 1. What governance controls (and nothing else does)

| Scope | Message | Notes |
|---|---|---|
| Every module parameter | `MsgUpdateParams` of each module (staking, slashing, distribution, mint, gov, feemarket, evm, erc20, policy, community, …) | authority = gov module account for all modules; bounds enforced by each module's `Validate()` |
| Treasury (community pool) | `MsgCommunityPoolSpend` | the protocol reserve + 2 % community tax + undistributed community budgets + false-alert slashes |
| Software upgrades | `MsgSoftwareUpgrade` / `MsgCancelUpgrade` | release flow in `docs/OPERATIONS.md`; rehearsed (`tests/upgrade/rehearsal.sh`) |
| Community dispute resolution | `MsgResolveAlert` | valid → refund; invalid → deposit + 10 % bond slashed (G-COMM-1) |
| Emergency message disabling | x/circuit `MsgTripCircuitBreaker` | **no authorised accounts** at genesis, so only a gov proposal can authorise or trip it |

**No admin keys exist.** Any future temporary privilege (e.g. a security council) must be added by a governance proposal, documented here with scope and expiry, and listed in `docs/security/key-inventory.md`.

## 2. Parameters (genesis template, proposed)

| Parameter | Value | Rationale |
|---|---|---|
| `min_deposit` | 10 000 NIRA | economic cost of a proposal |
| `min_initial_deposit_ratio` | 0.25 | anti-spam: 25 % must be posted at submission (G-GOV-1) |
| `expedited_min_deposit` | 50 000 NIRA | |
| `max_deposit_period` | 2 days (SDK default) | |
| `voting_period` | 5 days (template) | gives delegators and validators time to react; also the de-facto delay before changes |
| `expedited_voting_period` | 1 day | |
| `quorum` | 33.4 % | |
| `threshold` | 50 % | |
| `veto_threshold` | 33.4 % | NoWithVeto above this burns the deposit |
| `burn_vote_veto` / `burn_proposal_deposit_prevote` | SDK defaults (true / false) | |

## 3. Voting power
Stake-weighted: a delegator's vote overrides its validator's on its own stake; otherwise the validator's vote is inherited. EVM accounts vote through the gov precompile (tested: the vote is visible on the Cosmos side, `tests/cosmos-e2e/governance.sh`).

## 4. Attack analysis

| Attack | Defence | Residual |
|---|---|---|
| Proposal spam | deposit with 25 % upfront; veto burns deposit | — |
| Low-turnout capture | quorum 33.4 % of bonded stake | a coalition holding > 33.4 % turnout and a majority of it can pass proposals |
| Flash-loan / borrowed stake | voting power = bonded stake; bonding takes effect immediately, but exiting takes 21 days of slashable exposure | stake rented for 21+ days |
| Validator-inherited votes concentrate power | per-validator power cap (10 %) limits any single validator's inherited weight | delegators who don't vote delegate their voice |
| Malicious parameter | every parameter is bounded in code (e.g. power cap 3–33 %, community slash ≤ 50 %, min self-bond ≤ 1e9 NIRA) | values inside the bounds can still be harmful |
| Malicious upgrade | needs validators to run the new binary; old binaries halt instead of following | validators that blindly upgrade |
| Treasury drain | proposal plus voting period visible for 5 days | — |

## 5. Not implemented (proposed)
- A timelock between passing and execution, separate from the voting period (x/gov executes at the end of voting).
- A security council with narrowly scoped, expiring emergency powers (for example tripping the circuit breaker for EVM messages).
- Each needs a decision and an ADR before adding.
