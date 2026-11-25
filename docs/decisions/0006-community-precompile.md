# ADR-0006: x/community is reachable natively from both Cosmos and EVM

- Status: accepted
- Date: 2026-11-25
- Written by: design-spec
- Related: R-F11, ADR-0003, ADR-0005, docs/spec/community.md §10

## Context
Both interfaces must work natively. A Community Node operator using MetaMask, or a Solidity contract (a pool that operates nodes for many small holders), must be able to register, prove and deregister without a Cosmos wallet. The Cosmos CLI and gRPC path must work too.

## Options considered
| Option | Pros | Cons |
|---|---|---|
| A. Cosmos msgs only; EVM users sign Cosmos txs via EIP-712 | Least code | No access from contracts; poor EVM UX |
| B. Logic as a Solidity contract | EVM-native | Proof verification (ICS-23) and block hooks are awkward or expensive in the EVM; two sources of truth |
| C. Native module + stateful precompile at `0x…0900` calling the same keeper | One implementation and one state; both interfaces first-class | Precompile is consensus code: gas metering and determinism must be reviewed |

## Decision
C. The module is the single source of truth; the precompile is a thin ABI adapter over the keeper's msg server.

## Consequences
- Every operation gets e2e tests on both interfaces plus cross-checks.
- Precompile gas must cover the proof verification cost (P-COMM-proof_gas_per_byte, plus a base cost).
- Precompile bugs are chain-halting class → security review item.

## Revisit when
cosmos/evm v0.7 migration changes the precompile API.
