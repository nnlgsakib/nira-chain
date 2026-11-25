# ADR-0001: Sovereign PoS L1 with an off-finality community layer

- Status: accepted
- Date: 2026-11-25
- Written by: design-protocol
- Related: R-F1, R-F2, R-F5, T1–T7

## Context
The idea requires our own Core Validator role, a permissionless Community Node role with protocol-level rewards, native EVM, and our own fee and reward policy. It also requires that consensus never depends on community machines being online.

## Options considered
| Option | Pros | Cons |
|---|---|---|
| A. Contracts on an existing EVM chain | Cheapest, no validators to bootstrap | No own validator set, no protocol-level community role |
| B. L2 rollup | Inherits L1 security | Sequencer model; validator role is not a product primitive; community role only as a side system |
| C. Shared-security appchain (ICS / restaking) | Borrowed security | Validator selection controlled by the provider; conflicts with permissionless Core Validators |
| D. Sovereign PoS L1 + separate community layer | Full control of roles, economics and EVM integration | Must bootstrap a security budget and a validator ecosystem |

## Decision
D. The community layer is a separate module whose failure only affects community rewards, never finality.

## Consequences
- Security depends on the market value of staked tokens (tokenomics must model the cost of an attack).
- The community layer can evolve (new task types) without touching consensus.
- Hard to change later: the token is the staking asset; the BFT validator-set model.

## Revisit when
The security budget model shows the stake value needed for safety can't be reached in the first 12 months. Then reconsider shared security.
