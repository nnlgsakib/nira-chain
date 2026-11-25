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
| 4 | Validator management | ✅ | active set, rotation, min self-bond, power cap (live), jailing: `make validators-test` |
| 5 | Non-custodial delegated staking | ✅ | Delegate/undelegate via Cosmos tx and the EVM staking precompile, cross-checked (`tests/cosmos-e2e/e2e.sh`, `tests/evm-e2e/e2e.mjs`) |
| 6 | Reward distribution | 🟡 | x/distribution (F1) for validators and delegators. Community epoch settlement tested (INV-COMM-1, e2e payouts). *Validator/delegator reward claims not e2e tested; reward split not modelled (Q3)* |
| 7 | Slashing and unbonding | ✅ | Unbonding (staking + community), double-sign slash + tombstone + delegator propagation, downtime jail + unjail (`make validators-test`) |
| 8 | Native EVM execution | ✅ | cosmos/evm v0.6.0. Deploy, call, logs, revert, EIP-1559 and legacy txs (`tests/evm-e2e/e2e.mjs`) |
| 9 | Ethereum JSON-RPC | 🟡 | Exercised by the e2e suites: eth_chainId, blockNumber, getBalance, getCode, getBlockByNumber, getTransactionCount, gasPrice, call, estimateGas, sendRawTransaction, getTransactionByHash, getTransactionReceipt, getLogs, newFilter/newBlockFilter/newPendingTransactionFilter/getFilterChanges/uninstallFilter, feeHistory, syncing, net_version/listening, web3_clientVersion, txpool_status, eth_subscribe(newHeads). *No full Ethereum JSON-RPC conformance suite run yet* |
| 10 | Solidity deployment | ✅ | `tests/evm-e2e/Counter.sol` deployed and used through ethers.js |
| 11 | Community Node software | 🟡 | CLI client: `onion-init`, `register-node`, `prove`, `run`, `deregister-node` (Linux tested). *No GUI/desktop app; Windows/macOS untested; no resource limiting yet* |
| 12 | Independent verification | 🟡 | Random state-possession proofs (ICS-23) verified on-chain; advisory divergence alerts (ADR-0003). *Full re-execution verification, DA sampling and history serving are v2 task types (Q4)* |
| 13 | P2P networking | 🟡 | CometBFT p2p (PEX, persistent peers). *Sentry topology, peer limits and eclipse hardening not configured or tested* |
| 14 | Full node support | ✅ | Default node mode; all test networks |
| 15 | Archive node support | ✅ | `make sync-test`: archive serves historical state, pruned node refuses |
| 16 | Snapshot synchronisation | ✅ | `make sync-test`: state sync from a light-client-verified snapshot |
| 17 | Governance | ✅ | x/gov; module authorities = gov; e2e: param-change proposal (vote via CLI and EVM gov precompile), ResolveAlert slashing, software-upgrade rehearsal with binary switch |
| 18 | Monitoring / observability | 🟡 | Prometheus scrape + alert rules + compose stack (`deploy/monitoring`); on-chain security monitor (`scripts/monitor.py`, verified live). *No Grafana dashboards; rules not promtool-validated* |
| 19 | Security testing | 🟡 | Threat model (20 threats, evidence per threat), key inventory, fuzzing (`make fuzz`), adversarial unit + e2e tests (forged/copied/duplicate proofs, grinding, precompile atomicity, double sign), govulncheck. *No external audit, no runbooks yet* |
| 20 | Performance benchmarks | 🟡 | Baseline in `docs/benchmarks/` (native 707, EVM 2 037, mixed 1 030 TPS on 1 validator; 673 / 494 TPS mixed at 4 / 16 co-located validators; proof verification 9 µs). *Multi-host, soak and community-at-scale not measured* |
| 21 | Automated test infrastructure | 🟡 | `make test`, `make e2e`, `make localtest-check`. CI runs `make test` on PRs. *e2e not in CI yet* |
| 22 | Reproducible build | ✅ | `make release` (tarball + SHA256SUMS); `scripts/verify-reproducible.sh` builds from two clean clones → bit-identical. *Release signing and CI publishing pending* |
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
| Cosmos e2e | `make e2e-cosmos` | 8 + 6 |
| EVM e2e | `make e2e-evm` | 12 + 9 + 2 (incl. JSON-RPC filters/fee/net/WS, precompile atomicity) |
| Governance e2e | `make e2e-gov` | 2 (param change with EVM-precompile vote; false-alert slashing via proposal) |
| Upgrade rehearsal | `make upgrade-rehearsal` | 4 (old binary halts at plan height, new binary applies handler, state + EVM preserved) |
| Multi-validator + BFT | `make localtest && make localtest-check` | 8 |
| Sync + archive | `make sync-test` | 2 |
| Validator lifecycle (16 validators) | `make validators-test` | 7: rotation, live cap rejection (Cosmos + EVM), double-sign slash/tombstone/propagation, downtime jail + unjail, no fork |

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
