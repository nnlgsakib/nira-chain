# ADR-0005: Stack — Cosmos SDK 0.53 + CometBFT + cosmos/evm (Ignite)

- Status: accepted (module `x/nirachain` renamed to `x/policy` by ADR-0007)
- Date: 2026-11-25
- Written by: design-stack-selection (abbreviated: the user's constraint R-C1 fixes the base)
- Related: R-C1, R-F9–R-F12, ADR-0002

## Context
The repo was scaffolded with Ignite CLI (Cosmos SDK v0.53.6, CometBFT v0.38.21, ibc-go v10). The idea requires native EVM in the same state and the same block as native modules.

## Options considered
| Option | Pros | Cons |
|---|---|---|
| A. Cosmos SDK + cosmos/evm module (Ignite EVM app) | Single state machine; EVM shares bank balances; precompiles for staking/distribution/gov/bank; Ethereum JSON-RPC; maintained by Cosmos Labs; matches the existing scaffold | 18-decimal token; eth_secp256k1 keys; extra wiring complexity |
| B. Geth/reth-based EVM L1 with staking as system contracts | Maximum EVM equivalence | Throws away the scaffold; staking, slashing and community logic as contracts; Tendermint-style finality would need a custom engine |
| C. Separate EVM sidechain bridged to the Cosmos chain | Isolation | Explicitly forbidden by the idea (§6) |

## Decision
A. Native modules: auth, bank, staking, slashing, distribution, mint, gov, evidence, upgrade, plus `x/vm`, `x/feemarket`, `x/erc20`, `x/precisebank` (as wired by the cosmos/evm integration), plus our own `x/nirachain` (power cap, chain policy) and `x/community`.

Placement:
| Component | Surface | Location |
|---|---|---|
| CONS | CometBFT | config / genesis consensus params |
| STAKE | native module x/staking + slashing; cap hook in x/nirachain | app wiring, x/nirachain |
| EXEC | x/vm + feemarket + precompiles | app/ |
| TOKEN | x/mint + x/distribution + x/community budget | app, x/community |
| COMM | native module x/community | x/community |
| GOV | x/gov | app |

## Consequences
- Native token `anira` (18 decimals), display `nira`. Cosmos and EVM addresses are the same key (eth_secp256k1, coin type 60).
- EVM tracks the Ethereum hardforks supported by the cosmos/evm release.

## Revisit when
cosmos/evm stops being maintained, or EVM equivalence tests fail in ways that block common tooling.
