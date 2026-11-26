# Gap analysis — 2026-11-25

Method:
1. Check the chainskills production phase gates against the repo.
2. Compare the implementation with the master prompt (`docs/idea.md` §1–§19).
3. Review the risky code paths: precompile, community msg server, block hooks, simulation, dependencies.

Every gap has an ID, the evidence for it, what it blocks, and the skill that closes it.

**Severity:** **T** = blocks a public testnet · **M** = blocks mainnet · **I** = improvement.

## Fixed during this review

| ID | Gap | Fix | Evidence |
|---|---|---|---|
| G-SEC-1 | Onion reveal hashing in `SubmitTaskProof` was **not gas-metered**: up to `max_reveal_distance` SHA-256 iterations of free CPU per tx. Measured 0.26 ms at the default (4 096) and **6.2 ms at the parameter maximum (100 000)**, so a block of cheap failing txs could cost seconds | Reject distance > max before hashing; charge 72 gas per hash (the EVM SHA-256 precompile price) before computing | `TestRevealIsGasMetered`; spec §6.3 C5 v0.3 |
| G-TEST-1a | App simulation tests (`app/sim_test.go`) were **broken twice**: a hard-coded app name from before the rename, and an EVM panic (`denom metadata anira could not be found`) on every random genesis | App name fixed; `evmAppStateFn` injects the native-token metadata into the simulated genesis | `go test ./app -run TestFullAppSimulation -Enabled=true …` now runs blocks and operations (see G-TEST-1b) |
| G-DOC-1 | Architecture doc still described community miss counters and auto-pause, which the spec removed (selection is secret, so misses are unobservable) | architecture.md v0.2 | |
| G-DOC-2 | Spec said the app-hash ring holds `apphash_window` entries; the code keeps `window + epoch_blocks` (required so proofs stay verifiable for the whole epoch) | spec v0.3, INV-COMM-7 updated | |
| G-DOC-3 | Invariant registry lumped INV-COMM-1…10 into one row with no test mapping | `docs/spec/invariants.md`: every ID → test | |
| Q2 | EVM chain ID 47201 uniqueness unverified | Checked against chainid.network (2 909 chains): **unused**; registration still to do (G-OPS-5) | |

## Phase gates (production mode)

| Gate | Requirement | State |
|---|---|---|
| → spec | numeric requirements, trust assumptions, architecture ADR | ✅ (numbers marked *proposed*) |
| → stack | **every component has a spec** (state, ops, bounded params, invariants) | ❌ only COMM has one. STAKE (x/policy), TOKEN, GOV, NET have none (G-SPEC-1…4) |
| → build | stack ADR, placement, adapter | ✅ |
| → quality | rejection-path tests; spec ↔ code traceability | 🟡 COMM ✅, policy ✅ (no spec to trace to), registry now complete |
| → ops | threat model; testnet under chaos/load; critical findings closed | ❌ no threat model (G-SEC-2), no load/chaos runs (G-TEST-4) |
| → mainnet | readiness report with no blockers | ❌ not started |

**The project is in build, with quality work started. It can't formally pass the ops gate until G-SEC-2 and G-TEST-4 are closed.**

## Open gaps

### Specification and design

| ID | Gap | Sev | Evidence | Close with |
|---|---|---|---|---|
| G-SPEC-1 ✅ (docs/spec/policy.md) | x/policy (power cap) has an ADR but no component spec (state, params, failure cases) | M | `docs/spec/` has no `staking.md` | design-spec + protocol-staking-validators |
| G-SPEC-2 | **Tokenomics not specified or simulated.** The idea's 50/25/15/10 split isn't modelled: the validator/delegator split is set by commission (min 5 %), the reserve gets 2 % community tax plus undistributed community budget, and only the 15 % community share exists as a parameter. There is no security-budget or inflation simulation | M | no `docs/spec/tokenomics.md`; Q1/Q3 open | protocol-tokenomics |
| G-SPEC-3 ✅ v0.1 (docs/spec/governance.md; timelock/council proposed) | Governance not specified: no analysis of quorum/threshold attacks, no emergency policy. x/circuit is wired with no documented authority | M | no `docs/spec/governance.md` | protocol-governance |
| G-SPEC-4 ✅ (docs/spec/network.md + scripts/node-profile.py; sentry topology verified live in the Docker testnet: the sentried validator's only peer is its sentry, no other node peers with it, and it signs every block) | P2P/network: no topology (sentries, seeds), peer limits or eclipse hardening defined | T | CometBFT defaults | general knowledge (no protocol-p2p skill installed) |
| G-DES-1 ✅ DECIDED (ADR-0008, per-block for safety) | **Epoch-based validator management** (idea §2.1): the SDK updates the validator set every block, not per epoch | M | x/staking EndBlock | decision needed → ADR (per-block is safe; epochs add predictability) |
| G-DES-2 ✅ DECIDED (ADR-0008, performance gates eligibility) | **Selection ignores performance and slashing history** (idea §8): the active set is top-N by stake only | M | x/staking | decision needed → ADR (on-chain metrics only, or keep stake-only) |
| G-COMM-1 ✅ FIXED | **Community bonds are never slashed.** Now: a governance-rejected alert slashes 10 % of the bond (`false_alert_slash_bps`). "Penalties for provable misconduct" (idea §5) are limited to forfeiting alert deposits, because v1 has no provable fraud on the bond | M | spec §12 | ADR + design when v2 task types add fraud proofs |

### From the requirement compliance check (docs/COMPLIANCE.md)

| ID | Gap | Sev | Evidence | Close with |
|---|---|---|---|---|
| G-STAKE-1 ✅ FIXED | **No protocol-wide minimum self-bond** (idea §1A, §2.1, §8); each validator picks `min_self_delegation` ≥ 1 anira | M | staking params have no floor | x/policy `min_self_bond` + `AfterValidatorCreated` check (SDK jails below own min) |
| G-GOV-1 ✅ FIXED | `gov.min_initial_deposit_ratio = 0`: proposals can be opened with no upfront deposit (spam) | T | genesis params | set in network files (e.g. 0.25) |
| G-EVM-1 ✅ FIXED (txpool on dev/RPC nodes; filters/fee/net/WS in evm-e2e) | `txpool` namespace off by default; filter, fee and net methods respond (live probe) but aren't in automated tests | I | `api = "eth,net,web3"` | config + extend `evm-e2e` |

### Security

| ID | Gap | Sev | Evidence | Close with |
|---|---|---|---|---|
| G-SEC-2 ✅ v0.1 (docs/security/threat-model.md, key-inventory.md, runbooks.md; monitors in deploy/monitoring) | **No threat model document** covering the idea's §11 list (prevention, detection, recovery, test per threat) | T | — | quality-security |
| G-SEC-3 🟡 (fuzzers added; found + fixed a zero-window panic) | No fuzzing: MerkleProof decoding, precompile ABI decoding, msg handlers, genesis import | T | — | quality-testing / quality-security |
| G-SEC-4 ✅ FIXED (tests/evm-e2e/atomicity.mjs) | Precompile atomicity untested: an EVM call that invokes `0x…0900` and then **reverts** must leave no community state or escrow behind (the class of GO-2025-3684) | T | no test with a reverting contract | build-smart-contracts + quality-testing |
| G-SEC-5 | 4 dependency advisories without upstream fixes (K9) | I | `make govulncheck` | quality-security |
| G-SEC-6 ✅ CLOSED (not reproducible: errors are surfaced; root cause was the fixed precompile panic; e2e added) | `ExperimentalEVMMempool` in use; txs that fail CheckTx are dropped silently (K7) | T | `app/evm.go`, hang seen in e2e | stack-cosmos-evm |
| G-SEC-7 | Validator key management: remote signer / HSM not set up or tested | M | docs only | quality-security + ops-infrastructure |
| G-SEC-8 | No external audit of x/community, x/policy, the precompile and the genesis builder | M | — | quality-security |

### Consensus and staking

| ID | Gap | Sev | Evidence | Close with |
|---|---|---|---|---|
| G-CONS-1 🟡 | **Docker testnet (8 validators):** real network partitions verified: 6|2 → majority finalises, minority stuck, catches up on heal; 5|3 (≥ 1/3 cut off) → halt, then resume with no fork (`make testnet-test SCENARIO=partition`). 10-min soak under load passed (52 074 txs, no alerts, memory falls back when idle); 24 h soak still open. Earlier: **Tested now** (tests/consensus/check_validators.sh, 16 validators): double-sign evidence → slash + tombstone + delegator propagation, downtime jail/unjail, rotation. Still untested: real network partitions, long-running soak | T | tests cover 1/4 and 2/4 stopped only | quality-testing |
| G-CONS-2 ✅ FIXED (live via Cosmos tx + EVM precompile on 16 validators) | Live power-cap **rejection** never observed (needs ≥ 15 validators to activate) | T | Q7 | quality-testing (localtest N=16) |
| G-CONS-3 ✅ (scripts/trusted-checkpoint.py: requires ≥ 2 servers to agree on block hash + validator set, refuses with exit 2 on a forging server; used by tests/sync/check_sync.sh; procedure in runbooks RB-6) | Weak subjectivity: no trusted-checkpoint publishing or state-sync trust procedure | M | — | ops-launch |

### Community layer

| ID | Gap | Sev | Evidence | Close with |
|---|---|---|---|---|
| G-COMM-2 | Proof outsourcing to archive RPCs (K1); v2 local-data tasks (history serving, DA sampling) not built | M | ADR-0003 | protocol-verifiable-participation |
| G-COMM-3 🟡 | 6 containerised community nodes ran the prove loop unattended (30 tasks, all rewarded) in the Docker testnet. Node software is CLI-only. Reward tracking **done** (`rewards_total` via CLI, REST, precompile); still missing: resource monitoring/limits, installer, or Windows/macOS validation | T | `x/community/client/cli` | build-node-software |
| G-COMM-4 ✅ FIXED (epoch_work, task_done, app_hashes exported; TestGenesisPreservesInEpochWork) | Genesis export drops in-epoch work, so operators lose unsettled units across an export/import upgrade | I | `genesis.proto` comment | stack-cosmos-ignite |

### Testing, infrastructure, operations

| ID | Gap | Sev | Evidence | Close with |
|---|---|---|---|---|
| G-TEST-1b 🟡 | **Sweep 2026-11-26: 15/20 seeds pass 60 blocks, 0 fail, 5 skip** ("empty validator set" in the SDK mock while the real chain still had 134 bonded validators at the skip height: a harness-tracking divergence, root cause not isolated). 5 passing seeds run in CI. Earlier progress: TestFullAppSimulation passes 60 blocks for 6/10 seeds (1, 7, 13, 17, 19, 23) after fixing: realistic stakes (`stake_per_account` 1 M NIRA; base genesis accounts because the SDK's vesting generator overflows Int64 for 18 decimals), a floor-respecting create-validator op in x/policy, and **a real bug: x/policy initialised after x/staking, so any genesis with pre-populated, non-`exported` staking delegations (simulation, hand-built genesis) panicked on InitChain**. (Real `nirad export` output sets `exported`, so staking skips the hooks: verified against the pre-fix binary by `tests/upgrade/export_import.sh`.) Open: 4/10 seeds still empty the validator set (cause not yet determined); multi-app sims (import/export, determinism) are blocked by cosmos/evm process-global config. Original: Also, x/community and x/policy define **no simulation operations**, so the simulation never exercises Nira's modules | T | `-v` output: "skipping: empty validator set"; `WeightedOperations` returns empty | quality-testing |
| G-TEST-2 🟡 (e2e workflow added; status per GitHub run) | CI runs only `make test`; e2e, the 4-validator checks and govulncheck aren't in CI | T | `.github/workflows/go-unit.yml` | quality-testing |
| G-TEST-3 🟡 (Foundry automated: tests/foundry/run.sh 4/4 incl. precompile from a contract; Hardhat/MetaMask not automated, no formal conformance suite) | No Ethereum JSON-RPC conformance suite; Foundry, Hardhat and MetaMask flows not automated | T | | stack-cosmos-evm |
| G-TEST-4 🟡 BASELINE DONE (docs/benchmarks) — multi-host, soak, community-at-scale still open | **No performance or load tests** (R-N1 block time, R-N4 TPS, mixed native/EVM, community proof load) | T | | quality-testing |
| G-TEST-5 ✅ FIXED (tests/cosmos-e2e/governance.sh, upgrade rehearsal) | No governance e2e: param-change proposal, `MsgResolveAlert`, software-upgrade rehearsal | T | | quality-testing + ops-upgrades |
| G-OPS-1 ✅ FIXED (app/upgrades.go, tests/upgrade/rehearsal.sh) | **No upgrade-handler framework** (`app/upgrades/`); the first software upgrade has nothing to plug into | T | `grep SetUpgradeHandler` finds nothing | ops-upgrades |
| G-OPS-2 ✅ FIXED (tests/sync/check_sync.sh) | State-sync snapshots and archive node not configured or tested | T | | ops-infrastructure |
| G-OPS-3 ✅ (promtool-validated rules, Grafana dashboard, `testnet.py monitor` verifies all 8 rules healthy on 16 live nodes, every referenced metric has data, stopping a validator shows 12.5 % missing power; found + fixed a rule on a non-existent metric) | No monitoring dashboards or alerts | T | | ops-infrastructure |
| G-OPS-4 🟡 (make release + SHA256SUMS, bit-identical across clean clones; tag-only release workflow builds amd64+arm64, signs SHA256SUMS keylessly with cosign, drafts the GitHub release — not yet exercised: needs a first v* tag) | Release pipeline (Ignite action) never run; no signed checksums; reproducibility not verified | M | `.github/workflows/release.yml` | ops-launch |
| G-OPS-5 | Register EVM chain ID 47201 in ethereum-lists/chains | T | free as of 2026-11-25 | ops-launch |
| G-OPS-6 | No LICENSE file | T | — | owner decision |

## Recommended order

1. **Decisions (owner):** G-SPEC-2 economics (Q1/Q3), G-DES-1/2 (epochs, performance-aware selection), G-COMM-1 (community slashing), G-OPS-6 license.
2. **Before a public testnet:** G-SEC-2 threat model → G-SEC-4 precompile atomicity test → G-OPS-1 upgrade framework → G-CONS-1/2 (16-validator localtest: slashing, rotation, cap) → G-TEST-1b/2 (simulation ops, CI e2e) → G-SEC-6 mempool → G-TEST-4 baseline benchmarks → G-SPEC-4 network topology.
3. **Before mainnet:** remaining M items, external audit (G-SEC-8), readiness review.
