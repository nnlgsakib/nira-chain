# Project status

- Updated: 2026-11-25
- Phase: **build**. Development network only: **not audited, not production ready.**
- Source of truth for decisions and risks: [`CHAIN.md`](../CHAIN.md). This page tracks deliverables and remaining work.

Legend: ✅ done and tested · 🟡 partial or untested · ⬜ not started

## Deliverables (master prompt §17)

| # | Deliverable | Status | Evidence / what's missing |
|---|---|---|---|
| 1 | Blockchain source code | 🟡 | Cosmos SDK 0.53 app with x/policy, x/community and EVM. Tokenomics and some ops components still to come |
| 2 | Native execution | ✅ | `tests/cosmos-e2e/e2e.sh` (send, delegate, undelegate, params) |
| 3 | BFT consensus | ✅ | CometBFT 0.38, unmodified (ADR-0002). `tests/genesis/check_localtest.sh`: 1/4 down stays live, 2/4 down halts without forking, recovery with identical block hashes. *Network partitions and long-run stability not yet tested* |
| 4 | Validator management | 🟡 | x/staking active set (max 100), power cap (ADR-0004, unit tests INV-STAKE-CAP-1..3). *Live cap rejection needs ≥ 15 validators; rotation and jailing not e2e tested* |
| 5 | Non-custodial delegated staking | ✅ | Delegate/undelegate via Cosmos tx and the EVM staking precompile, cross-checked (`tests/cosmos-e2e/e2e.sh`, `tests/evm-e2e/e2e.mjs`) |
| 6 | Reward distribution | 🟡 | x/distribution (F1) for validators and delegators. Community epoch settlement tested (INV-COMM-1, e2e payouts). *Validator/delegator reward claims not e2e tested; reward split not modelled (Q3)* |
| 7 | Slashing and unbonding | 🟡 | Unbonding tested (staking and community bonds). Slashing params set. *Double-sign and downtime slashing not yet exercised in a test network* |
| 8 | Native EVM execution | ✅ | cosmos/evm v0.6.0. Deploy, call, logs, revert, EIP-1559 and legacy txs (`tests/evm-e2e/e2e.mjs`) |
| 9 | Ethereum JSON-RPC | 🟡 | Exercised by the e2e suites: eth_chainId, blockNumber, getBalance, getCode, getBlockByNumber, getTransactionCount, gasPrice, call, estimateGas, sendRawTransaction, getTransactionByHash, getTransactionReceipt, getLogs. *No full Ethereum JSON-RPC conformance suite run yet* |
| 10 | Solidity deployment | ✅ | `tests/evm-e2e/Counter.sol` deployed and used through ethers.js |
| 11 | Community Node software | 🟡 | CLI client: `onion-init`, `register-node`, `prove`, `run`, `deregister-node` (Linux tested). *No GUI/desktop app; Windows/macOS untested; no resource limiting yet* |
| 12 | Independent verification | 🟡 | Random state-possession proofs (ICS-23) verified on-chain; advisory divergence alerts (ADR-0003). *Full re-execution verification, DA sampling and history serving are v2 task types (Q4)* |
| 13 | P2P networking | 🟡 | CometBFT p2p (PEX, persistent peers). *Sentry topology, peer limits and eclipse hardening not configured or tested* |
| 14 | Full node support | ✅ | Default node mode; all test networks |
| 15 | Archive node support | 🟡 | `pruning = "nothing"` documented. *Not tested* |
| 16 | Snapshot synchronisation | 🟡 | SDK snapshots + CometBFT state sync available. *Not configured or tested* |
| 17 | Governance | 🟡 | x/gov with params set in the genesis network file; module authorities = gov. *No proposal lifecycle e2e test yet (param change, ResolveAlert, upgrade)* |
| 18 | Monitoring / observability | 🟡 | Prometheus endpoints available (`prometheus = true`); typed module events. *No dashboards or alerts* |
| 19 | Security testing | 🟡 | Adversarial unit tests (forged proofs, grinding, copying, duplicate pay, hook failure isolation); determinism audit (0 findings); govulncheck. *No threat model document, no fuzzing, no external audit* |
| 20 | Performance benchmarks | ⬜ | Not started (R-N1/R-N4 targets unverified) |
| 21 | Automated test infrastructure | 🟡 | `make test`, `make e2e`, `make localtest-check`. CI runs `make test` on PRs. *e2e not in CI yet* |
| 22 | Reproducible build | 🟡 | `-trimpath`, `go mod verify`, pinned deps, Makefile. *Bit-for-bit reproducibility not verified; release workflow (Ignite) not exercised* |
| 23 | Deployment documentation | 🟡 | `genesis/README.md` (ceremony), `docs/OPERATIONS.md`. *No infrastructure-as-code* |
| 24 | Node operator documentation | 🟡 | `docs/OPERATIONS.md`, `docs/INSTALL.md` |
| 25 | Developer documentation | 🟡 | `readme.md`, `precompiles/community/ICommunity.sol`, specs in `docs/spec/`. *No dApp tutorial or JS SDK* |
| 26 | Mainnet readiness assessment | ⬜ | Not started (`docs/readiness.md`) |

## Acceptance criteria (master prompt §19)

| Area | Criterion | Status |
|---|---|---|
| Consensus | Independent validators form a network | ✅ 4 validators, separate keys and gentxs |
| | Blocks proposed and finalised; voting power correct | ✅ all 4 sign; equal power from the network file |
| | Validator rotation | ⬜ not tested |
| | Double signing handled | ⬜ not tested (SDK evidence module present) |
| | Partitions don't violate safety | 🟡 halt-not-fork tested with 2/4 stopped; real partition not simulated |
| | Failures within fault tolerance | ✅ 1/4 down stays live |
| Staking | Self-bond, delegate, non-custodial, unbond | ✅ |
| | Rewards and commission correct | 🟡 not asserted numerically |
| | Slashing works | ⬜ |
| | Concentration protections | ✅ unit (randomised); 🟡 live rejection pending (≥ 15 validators) |
| Community | Join without permission; disconnect/reconnect | ✅ (offline = no penalty; onion rotation) |
| | Independent verification; tasks verifiable; fake work rejected | ✅ real ICS-23 proofs; forged/copied/duplicate proofs rejected |
| | Not a consensus dependency | ✅ hooks never error (INV-COMM-8); chain runs with zero community nodes |
| | Ordinary computers can run nodes | 🟡 Linux only; resource use not measured |
| EVM | Deploy, execute, txs, RPC, gas, receipts, logs, state consistency | ✅ (bank balance == EVM balance on both paths) |
| | Common developer tools | 🟡 ethers.js tested; MetaMask/Foundry/Hardhat documented, not automated |
| Performance | Benchmarks, mixed load, scaling, sync, long-run | ⬜ |
| Security | Threat model, attack tests, economic simulation, independent review | 🟡 attack tests in unit suites; ⬜ threat model, simulation, review |

## Test inventory

| Suite | Command | Checks |
|---|---|---|
| Go unit + invariants | `make test-unit` | 42 test functions: power cap (randomised 2000 ops), community (real IAVL proofs, every rejection path, randomised 120 blocks), spec test vectors |
| Genesis builder | `make test-genesis` | 11 |
| Cosmos e2e | `make e2e-cosmos` | 7 + 6 |
| EVM e2e | `make e2e-evm` | 10 + 9 |
| Multi-validator + BFT | `make localtest && make localtest-check` | 8 |

## Known dependency vulnerabilities (govulncheck, 2026-11-25)

7 were reachable; 4 remain after patch upgrades (`684fce2`):

| ID | Module | Why still open |
|---|---|---|
| GO-2025-3684 | cosmos/evm | **False positive.** The fix commit `0fff8c1` is an ancestor of v0.6.0; the advisory has no fixed version recorded |
| GO-2026-6278 | gorilla/websocket v1.5.3 | Weak PRNG for WebSocket masking (client side). Fixed only on an untagged commit; revisit on the next tagged release |
| GO-2026-5932 | golang.org/x/crypto/openpgp | Package unmaintained, no fix; transitive. Remove the import path when upstream drops it |
| GO-2026-4479 | pion/dtls v2 | Fixed only in v3; transitive dependency |

## Remaining work (prioritised)

Detailed, evidence-based list with IDs: [GAPS.md](GAPS.md).

1. **Economics** (Q1/Q3): tokenomics spec and simulation of the values in `genesis/networks/*.toml` (inflation band, 15 % community share, fee floor, validator/delegator returns). The final allocation is a decision for the project owner.
2. **Threat model** (quality-security): all the master prompt's §11 threats with prevention, detection and recovery. Then fuzzing of the proof decoder and precompile ABI, and an external audit of x/community, x/policy and the precompile.
3. **Consensus/staking tests**: ≥ 15-validator network (live cap rejection), double-sign and downtime slashing, validator rotation, partition simulation, long-running soak.
4. **Governance e2e**: param-change proposals, `MsgResolveAlert`, a software-upgrade rehearsal with an upgrade handler.
5. **Performance**: benchmark plan and baseline (native, EVM, mixed; block time at 4/16/50 validators; community proof load).
6. **Sync and storage**: state-sync snapshots, an archive node, community node resource limits.
7. **Community layer v2** (Q4): history serving and DA sampling tasks to reduce the outsourcing risk (K1).
8. **CI**: run `make e2e` and `make localtest-check` on PRs; release pipeline with checksums.
9. **Ops**: monitoring dashboards and alerts, sentry/validator reference topology, runbooks.
10. **Mainnet readiness** review (`docs/readiness.md`).
