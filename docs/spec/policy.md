# Spec — STAKE-POLICY: `x/policy` (voting-power cap, minimum self-bond)

- Version 0.1 (2026-11-25). Status: specified, built, tested. ADR-0004, ADR-0008, G-STAKE-1.
- Mechanism skill: protocol-staking-validators. Surface: native module `x/policy` (staking hooks). APIs: REST `/nira/policy/v1/params`, CLI `nirad q policy params`.

## 1. Purpose
Adds two chain-wide rules on top of x/staking, without changing how voting power is computed (power = bonded stake, so it stays fully slashable):
1. **Voting-power cap:** no delegation may push a validator above `max_validator_power_bps` of total bonded stake.
2. **Minimum self-bond:** every validator must be created with `min_self_delegation ≥ min_self_bond`.

## 2. State

| Entity | Key | Value | Writer | Bound |
|---|---|---|---|---|
| Params | — | §4 | gov (`MsgUpdateParams`) | 1 |
| PreDelegationTokens | prefix 0x01 ‖ validator address | validator tokens before the current delegation change | staking hooks | ≤ 1 entry per validator, and **0 between transactions** (INV-STAKE-CAP-3) |

The scratch entry lives in the KV store, not a transient store, because EVM precompiles run on a cache built only from KV stores (found by e2e, K5).

## 3. Operations and hooks

| Trigger | Logic |
|---|---|
| `MsgUpdateParams{authority, params}` | authority must equal the gov module account; `params.Validate()` |
| `BeforeDelegationCreated` / `BeforeDelegationSharesModified(val)` | record `val.tokens` |
| `AfterDelegationModified(val)` | pop the record. If `tokens_after ≤ tokens_before` (undelegation path), allow. Otherwise run **CapCheck** |
| `BeforeDelegationRemoved(val)` | pop the record (full undelegation) |
| `AfterValidatorCreated(val)` | **MinSelfBondCheck** |

**CapCheck(val):**
1. `total = TotalBondedTokens + (val.tokens if val is not bonded)`.
2. Skip if `total < cap_activation_bonded` or `#LastValidators < cap_activation_min_validators` (bootstrap).
3. Reject with `ErrValidatorPowerCap` (code 1101) iff `val.tokens · 10 000 > max_validator_power_bps · total` (exact integers).

**MinSelfBondCheck(val):** reject with `ErrMinSelfBond` (code 1102) iff `val.min_self_delegation < min_self_bond`.

x/staking calls these hooks on every path (Cosmos tx, authz, gentx, redelegation via `BeginRedelegation → Unbond → Delegate`, and the EVM staking precompile `0x…0800`). An error reverts the whole tx or EVM frame.

## 4. Parameters

| ID | Type | Default (proposed) | Bounds | Effect of change at runtime |
|---|---|---|---|---|
| P-STAKE-max_validator_power_bps | u32 | 1 000 (10 %) | 300–3 333 | applies to future delegations only |
| P-STAKE-cap_activation_bonded | Int | 5 000 000 NIRA | ≥ 0 | |
| P-STAKE-cap_activation_min_validators | u32 | 15 | ⌈10 000 / bps⌉ – 1 000 | ensures the cap is always satisfiable |
| P-STAKE-min_self_bond | Int | 100 000 NIRA | 1 – 1e9 NIRA | applies to validators created afterwards; existing validators keep their own (only-increasing) `min_self_delegation` |

## 5. Invariants

| ID | Statement | Test |
|---|---|---|
| INV-STAKE-CAP-1 | after an accepted delegation with the cap active, `tokens · 10 000 ≤ bps · total` | `power_cap_test.go` (randomised), `check_validators.sh` #2 |
| INV-STAKE-CAP-2 | undelegation never fails because of the cap | unit, `e2e.sh` #5, EVM precompile undelegate |
| INV-STAKE-CAP-3 | no PreDelegationTokens entry remains after a staking call | `TestINV_STAKE_CAP_3_NoResidue` |
| INV-STAKE-SELF-1 | every validator was created with `min_self_delegation ≥ min_self_bond` | unit, `e2e.sh` #8, EVM createValidator |

## 6. Failure and adversarial cases

| Case | Behaviour |
|---|---|
| Others unbond, leaving a validator above the cap | the validator is not forced down; it just can't grow (documented, ADR-0004) |
| One operator runs many validators | not detectable on chain (K3). Min self-bond raises the cost per identity |
| Validator lowers its self-bond below its own min | SDK jails it (`min_self_delegation` can only increase) |
| Params missing at genesis | `x/policy` initialises before genutil, so gentxs see the params (regression found in e2e) |
