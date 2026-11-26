# Nira Chain

> A sovereign PoS L1 where bonded Core Validators finalise blocks with BFT consensus, native EVM runs in the same state, and permissionless Community Nodes earn only for cryptographically verified work.

## Status
- Mode: production
- Phase: quality (build complete for CONS/STAKE/EXEC/COMM/GOV; threat model v0.1; testnets under fault, sync, upgrade and load tests). Ops gate still open: external audit, multi-host testnet, runbooks
- Last updated: 2026-11-26 by blockchain-dev (remaining-tasks round: K7 closed, monitoring verified, community status + genesis export, checkpoints, runbooks, Foundry, release workflow, remote signing)
- Next step: owner decisions (economics Q1/Q3, license, chain-ID registration G-OPS-5); then first v* tag to exercise the release workflow; external audit

## Idea
Full text: `docs/idea.md`. Hybrid L1: professional Core Validators run Tendermint-style BFT consensus with delegated, non-custodial PoS. Ordinary home machines run Community Nodes that do randomly assigned, verifiable work and never sit on the finality path. Native EVM with Ethereum JSON-RPC shares one state with native modules. Rewards split between capital, validator responsibility and verified community service, with governed issuance.

## Requirements
### Functional
- R-F1 Permissionless validator creation; active set = top-N by bonded power, min self-bond
- R-F2 Non-custodial delegation, redelegation, undelegation with unbonding; rewards + commission
- R-F3 Slashing + jailing for double sign and downtime, propagated to delegators
- R-F4 Per-validator voting-power cap (ADR-0004)
- R-F5 Permissionless Community Node registration with bond
- R-F6 Unpredictable, verifiable task assignment to Community Nodes
- R-F7 Community rewards only for on-chain-verified work
- R-F8 Divergence alerts from Community Nodes, advisory and off the finality path
- R-F9 Ethereum txs (legacy, EIP-2930, EIP-1559), Solidity deploy/call, logs, receipts
- R-F10 Ethereum JSON-RPC (eth_, net_, web3_, filters) on full nodes
- R-F11 One account/key model and one balance for native and EVM
- R-F12 On-chain governance over params, treasury and upgrades; no admin key
- R-F13 State sync snapshots; pruned (community) and archive modes
### Non-functional (proposed unless stated)
- R-N1 Block time ≤ 2 s p50, ≤ 4 s p95 at 100 validators (proposed; benchmark pending)
- R-N2 Finality = 1 block (BFT)
- R-N3 Active validators ≤ 100 at genesis params (governable up to 150)
- R-N4 ≥ 300 TPS simple native/EVM transfers on reference hardware (proposed baseline, benchmark pending)
- R-N5 Community node: 4 cores / 8 GB / 500 GB SSD / 25 Mbps, pruned; state sync to head < 30 min
- R-N6 Community layer outage → 0 impact on block production
### Constraints
- R-C1 Cosmos SDK v0.53 + CometBFT v0.38 Ignite scaffold already exists (Go)
- R-C2 EVM must be in-state, not a sidechain
### Non-goals
- Community nodes voting in consensus; fixed APY promises; custom consensus algorithms; a bridge between native and EVM state

## Architecture
Detail: `docs/spec/architecture.md`
- Architecture class: sovereign PoS L1 + off-finality community participation layer (ADR-0001)
- Node roles: Core Validator, Delegator, Community Node, Full Node, Archive Node
- Trust assumptions: < 1/3 Byzantine power (safety), > 2/3 online (liveness, else halt), weak subjectivity within unbonding period

## Stack
- Framework + versions: Cosmos SDK v0.53.6, CometBFT v0.38.21, ibc-go v10.4.0, cosmos/evm v0.6.0 (go-ethereum → cosmos/go-ethereum v1.16.2-cosmos-1), Go 1.26, EVM chain ID 47201 (proposed, Q2)
- Adapter skill: stack-cosmos-ignite + stack-cosmos-evm
- Decision: ADR-0005

| Component | Surface | Location | ADR |
|---|---|---|---|
| CONS | CometBFT engine | config, genesis consensus params | ADR-0002 |
| STAKE | native x/staking, x/slashing; cap hook | app/, x/policy | ADR-0004 |
| EXEC | x/vm, x/feemarket, x/erc20, precompiles | app/ | ADR-0005 |
| TOKEN | x/mint, x/distribution, community budget | app/, x/community | — |
| COMM | native module | x/community | ADR-0003 |
| GOV | x/gov | app/ | — |

## Components
| Component | protocol-* skill | Spec | Status | Evidence |
|---|---|---|---|---|
| CONS | protocol-consensus | architecture.md §1,6 | tested (4-validator net) | tests/genesis/check_localtest.sh: 1/4 down live, 2/4 down halts, recovery with identical hashes |
| STAKE | protocol-staking-validators | docs/spec/policy.md | tested (unit + localnet) | x/policy/keeper/power_cap_test.go (INV-STAKE-CAP-1..3, randomised 2000 ops); tests/cosmos-e2e (MsgDelegate/Undelegate), tests/evm-e2e (staking precompile delegate/undelegate, createValidator min self-bond); live cap rejection + slashing + rotation on 16 validators (tests/consensus/check_validators.sh) |
| EXEC | protocol-state-execution | ADR-0005 | tested (localnet) | tests/evm-e2e/e2e.mjs — 9/9 pass: deploy, logs/filters, receipts, revert, EIP-1559 + legacy tx, bank==EVM balance, staking precompile |
| TOKEN | protocol-tokenomics | missing | designed | |
| COMM | protocol-verifiable-participation | docs/spec/community.md v0.2 | tested (unit + localnet, Cosmos and EVM) | x/community/keeper/community_test.go (real IAVL proofs, all C-checks, INV-COMM-1..10 randomised); tests/cosmos-e2e/community.sh 6/6 (CLI client); tests/evm-e2e/community.mjs 9/9 via precompile 0x…0900 with Cosmos REST cross-checks |
| RAND | protocol-randomness | community.md §5 | tested | x/community/types/derive_test.go (spec vectors, INV-COMM-9) |
| GOV | protocol-governance | docs/spec/governance.md | tested | tests/cosmos-e2e/governance.sh (param change, EVM vote, ResolveAlert), tests/upgrade/rehearsal.sh |
| NET | — (no skill) | docs/spec/network.md | built (profiles) | scripts/node-profile.py; sync tests; live sentry test pending |

## Decisions
| ADR | Title | Status |
|---|---|---|
| ADR-0001 | Sovereign PoS L1 with off-finality community layer | accepted |
| ADR-0002 | Tendermint-family BFT, bounded active set | accepted |
| ADR-0003 | Community layer: hash-onion sortition + ICS-23 state proofs | accepted |
| ADR-0004 | Per-validator voting-power cap at delegation time | accepted |
| ADR-0005 | Stack: Cosmos SDK + cosmos/evm via Ignite | accepted |
| ADR-0007 | Nira naming for chain-owned APIs, modules and binary | accepted |
| ADR-0009 | Community v2: no storage-proof tasks for consumer nodes (keep v1) | accepted |
| ADR-0008 | Validator-set cadence (per-block) and performance-aware eligibility | accepted |
| ADR-0006 | x/community reachable from Cosmos and EVM (precompile 0x…0900) | accepted |

## Open questions
Full gap analysis: docs/GAPS.md (IDs G-*). Requirement-by-requirement check: docs/COMPLIANCE.md (81 met+tested, 18 met untested, 17 partial, 3 deviations decided in ADR-0008, 5 missing; economics excluded).
- [x] Q8 Decided per-block (ADR-0008); owner may override
- [x] Q9 Decided: performance gates eligibility, power = stake (ADR-0008); owner may override
- [x] Q10 Done: false-alert bond slashing (G-COMM-1)
- [ ] Q7 Live power-cap *rejection* needs ≥ 15 validators (cap activation); 4-validator ceremony + BFT fault tests done (tests/genesis/check_localtest.sh)
- [ ] Q6 Dev bootstrap uses scripts/localnet.sh (eth_secp256k1 keys); `ignite chain serve` creates plain secp256k1 keys that can't sign EVM txs — don't use it for EVM testing
- [ ] Q1 Token supply, genesis allocation and vesting — now data-driven: edit genesis/networks/<net>.toml (template: mainnet.example.toml, all values proposed). Decision still needed from the user before mainnet
- [x] Q2 EVM chain ID 47201: unused in chainid.network registry (checked 2026-11-25); register it before testnet (G-OPS-5)
- [ ] Q3 Reward split 50/25/15/10 from the idea: validators vs delegators is set by commission in the F1 model, not by protocol. Needs a tokenomics spec (protocol-tokenomics)
- [x] Q4 Decided (ADR-0009, option A): no v2 storage tasks for consumer nodes; v1 rewards proven, correct, live state access, not storage
- [ ] Q5 Formal model of x/community settlement (quality-formal-verification)

## Risks
| ID | Risk | Likelihood | Impact | Mitigation | Owner skill |
|---|---|---|---|---|---|
| K1 | Community nodes can fetch proofs from public archive RPCs (they prove state access, not storage) | high | low (accepted, ADR-0009) | Documented: rewards pay for correct live state access by a bonded node; storage would need an audited PoRep tiered limitation | protocol-verifiable-participation |
| K2 | Proposer biases the epoch seed | low | low | Hash-onion per-node reveal; reward-fairness impact only | protocol-randomness |
| K3 | One operator runs several validators to bypass the cap | medium | high | Monitoring, operator disclosure, delegator UI | protocol-staking-validators |
| K4 | > 1/3 stake on one cloud provider | medium | high | Diversity monitoring, validator docs | ops-infrastructure |
| K5 | cosmos/evm wiring bugs (precompile + staking hook interaction) | medium | high | Found + fixed: transient stores are invisible inside EVM precompiles (hooks must use KV stores). Both-interface e2e suites run per change | quality-testing |
| K6 | Base fee decays to ~0 on empty blocks and feemarket min_gas_price = 0 → near-free spam | high | medium | Network template sets params.feemarket.min_gas_price = 10 gwei (proposed); operators must set minimum-gas-prices (genesis/README.md) | protocol-state-execution |
| K9 | 4 reachable dependency advisories without an upstream fix (gorilla/websocket, x/crypto/openpgp, pion/dtls v2; cosmos/evm GO-2025-3684 is a false positive) | low | medium | Tracked in docs/STATUS.md; rescan with `make govulncheck` on every dependency change | quality-security |
| K8 | ICS-23 can't prove non-membership next to empty-value leaves (collections KeySets) or in empty stores; sample stores must avoid them | medium | medium | Defaults acc/evm/distribution/slashing (200/200 verified live); tools/proofprobe check before changing; SDK upgrades may add KeySets to these stores → re-probe per upgrade | protocol-verifiable-participation |
| K7 | ~~EVM txs silently dropped~~ **Closed:** probing shows CheckTx failures are returned to the RPC caller (fee floor, intrinsic gas, nonce, funds) and execution failures are mined with status 0 (e2e). The original hang was the precompile transient-store panic, fixed. Difference from Ethereum: failed/cheap txs pay ≥ 50 % of gasLimit (feemarket min_gas_multiplier) | low | low | e2e `invalid txs are rejected…`; document for wallet devs | stack-cosmos-evm |

## Genesis
Network definition (supply, allocations, vesting, initial validators and community nodes, all economic params): `genesis/networks/<net>.toml` → `scripts/genesis/build_genesis.py check|pregenesis|finalize`. Runbook: `genesis/README.md`.

## How to test (both interfaces)
`scripts/localnet.sh && nirad start` then `tests/cosmos-e2e/e2e.sh`, `tests/cosmos-e2e/community.sh`, and with `KEY=$(echo y | nirad keys unsafe-export-eth-key dev1 --keyring-backend test)`: `node tests/evm-e2e/e2e.mjs`, `node tests/evm-e2e/community.mjs`; unit: `go test ./x/... ./app/...`

## Docs
[readme](readme.md) · [install](docs/INSTALL.md) · [operations](docs/OPERATIONS.md) · [status & remaining work](docs/STATUS.md) · [gap analysis](docs/GAPS.md) · [requirement compliance](docs/COMPLIANCE.md) · [threat model](docs/security/threat-model.md) · [benchmarks](docs/benchmarks/README.md) · [genesis](genesis/README.md)

## Production readiness
Not started. See docs/readiness.md (to be written by ops-production-readiness).
