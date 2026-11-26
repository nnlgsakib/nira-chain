# ADR-0007: Nira naming for chain-owned APIs, modules and binary

- Status: accepted
- Date: 2026-11-25
- Written by: blockchain-dev
- Related: ADR-0004, ADR-0005, ADR-0006

## Context
The Ignite scaffold names leaked into public APIs: REST `/nira-chain/nirachain/v1/...`, proto packages `nirachain.nirachain.v1`, module `x/nirachain`, binary `nira-chaind`. Proto package names are part of the API: REST paths, gRPC service names, and the type URLs inside signed transactions. After mainnet they can't change without breaking every client. Before launch, renaming them is free.

## Options considered
| Option | Pros | Cons |
|---|---|---|
| A. Rename everything to `nira`, including SDK APIs (`/cosmos/...` → `/nira/...`) | Uniform branding | Breaks Keplr, Leap, CosmJS, explorers, indexers, IBC relayers; forks of the SDK; type URLs in txs differ from the standard |
| B. Rename only what Nira owns; keep the standard Cosmos SDK and cosmos/evm namespaces | Clean Nira APIs; full ecosystem compatibility | Mixed prefixes (`/cosmos/...` for standard modules, `/nira/...` for Nira modules), like Ethereum chains keeping `eth_*` |
| C. Keep scaffold names | No work | Permanent awkward public names |

## Decision
B.
- Proto packages: `nira.community.v1` and `nira.policy.v1`. REST: `/nira/community/v1/...` and `/nira/policy/v1/...`.
- Module `x/nirachain` becomes `x/policy` (genesis key `policy`, CLI `nirad q policy …`).
- Amino names: `nira/community/...` and `nira/policy/...`.
- Binary `nira-chaind` becomes `nirad`; default home `~/.nira-chain` becomes `~/.nira`.

Standard modules (`/cosmos/bank/...`, `/cosmos/staking/...`, `/cosmos/evm/...`) keep their names: they are the protocol standard that wallets and tools implement.

## Consequences
- These names are frozen at mainnet; any later change needs new versioned packages (`nira.community.v2`).
- The Go module path `nira-chain` is internal (never exposed on chain) and stays.
