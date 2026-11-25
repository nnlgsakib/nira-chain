# Requirement compliance — does the logic meet the master prompt?

- Date: 2026-11-25 · Source: `docs/idea.md` §1–§13, §19
- **Out of scope by request:** §7 token distribution and reward split (allocation percentages, inflation, sustainability). Those are tracked as G-SPEC-2.
- Method: each requirement is checked against the code path that implements it. Evidence is a file, a test, or a live probe on a running node. Gaps reference `docs/GAPS.md`.

| Verdict | Meaning |
|---|---|
| ✅ | Meets: logic in place **and** exercised by our tests |
| 🟢 | Meets by logic (upstream CometBFT/SDK/cosmos-evm code path confirmed) but **not yet exercised** in our tests |
| 🟡 | Partially meets |
| 🔶 | Deliberate deviation: design choice that needs owner confirmation |
| ❌ | Missing |

## Summary

| Section | ✅ | 🟢 | 🟡 | 🔶 | ❌ |
|---|---:|---:|---:|---:|---:|
| §1 / §2.1 Core Validator Network | 7 | 5 | 1 | 1 | 1 |
| §2.2 Community Node Network | 8 | 1 | 1 | 0 | 1 |
| §2.3 Full nodes, archive nodes, sync | 1 | 2 | 1 | 0 | 0 |
| §3 Consensus and finality | 6 | 5 | 1 | 0 | 0 |
| §4 Non-custodial staking and delegation | 9 | 4 | 0 | 0 | 0 |
| §5 Community verification layer | 11 | 0 | 3 | 0 | 0 |
| §6 Native EVM | 13 | 2 | 3 | 0 | 0 |
| §8 Validator selection and decentralisation | 6 | 0 | 1 | 2 | 2 |
| §9 Networking, §10 Storage | 1 | 6 | 3 | 0 | 1 |
| §12 Governance | 1 | 4 | 2 | 0 | 0 |
| §13 Node software | 2 | 0 | 4 | 0 | 2 |
| **Total** | **65** | **29** | **20** | **3** | **7** |

**Verdict:** the core logic meets the prompt for consensus safety, non-custodial staking, the native EVM, and a non-decorative, sybil-neutral community verification layer. Four logic gaps contradict explicit requirements:
1. **No protocol-wide minimum self-bond** (§1A, §2.1): ❌, cheap to fix (G-STAKE-1).
2. **Validator selection ignores performance and slashing history** (§8): 🔶, owner decision (G-DES-2).
3. **Validator-set changes are per block, not per epoch** (§2.1): 🔶, owner decision (G-DES-1).
4. **Community penalties cover alert deposits only, never the bond** (§5): 🟡 (G-COMM-1).

## §1 / §2.1 Core Validator Network

| Requirement | Verdict | Evidence / logic |
|---|---|---|
| Permissionless validator registration | 🟢 | `MsgCreateValidator` (Cosmos tx and staking precompile `0x…0800`), no allow-list |
| **Minimum validator self-bond** / "significant self-bonded stake" | ✅ | **Fixed:** x/policy `min_self_bond` (100 000 NIRA) checked in `AfterValidatorCreated` for every creation path; SDK keeps self-bond ≥ min or jails. Tests: unit, Cosmos e2e #8, EVM precompile. Previously ❌: x/staking has **no chain-wide floor**. Each validator sets its own `min_self_delegation` (≥ 1 anira). Params: `unbonding_time, max_validators, max_entries, historical_entries, bond_denom, min_commission_rate` only. **G-STAKE-1** |
| Delegated voting power | ✅ | power = bonded tokens / 10¹⁸; delegations add power (`tests/cosmos-e2e/e2e.sh`, EVM precompile test) |
| Active validator selection | ✅ | top `max_validators` by power each EndBlock; 4-validator net all bonded (`check_localtest.sh`) |
| Epoch-based validator management | 🔶 | Decided per-block for safety (ADR-0008): misbehaving validators lose power within 2 blocks. Epochs used for the community layer and rewards |
| Validator rotation | 🟢 | Ranking changes from stake, jailing and unbonding rotate the set at EndBlock. No forced rotation. Not tested (G-CONS-1) |
| Block proposal / voting / finality | ✅ | CometBFT, unmodified (ADR-0002); all 4 validators sign each block |
| Deterministic execution | ✅ | determinism audit 0 findings; identical block hashes across 4 nodes after recovery |
| Double-sign protection | 🟢 | x/evidence + x/slashing: 5 % slash, tombstone. Not exercised (G-CONS-1) |
| Slashing / jailing | 🟢 | double-sign 5 %; downtime 0.01 % + 600 s jail below 5 % signed of 10 000 blocks. Not exercised |
| Validator unbonding | ✅ | 21-day unbonding; unbonding entries verified (`e2e.sh` #5) |
| Performance tracking | 🟢 | x/slashing signing info (missed blocks per window): `nirad q slashing signing-info` |
| Manageable set size / expandable | ✅ | `max_validators` param (100), governance-changeable without a redesign |
| No single-operator dependence | 🟡 | Genesis builder rejects any validator ≥ 1/3 power and requires ≥ 4. **No protocol defence against one operator running many validators** (K3) |
| Concentration control (§4) | ✅ | per-validator cap (x/policy, ADR-0004), see §4 |

## §2.2 Community Node Network

| Requirement | Verdict | Evidence / logic |
|---|---|---|
| Join without permission | ✅ | `MsgRegisterNode` with bond; e2e both interfaces |
| Discover peers / synchronise | 🟢 | Community nodes are ordinary CometBFT full nodes (PEX, block sync) |
| Verify finalised blocks independently | 🟡 | Running a full node verifies every block, but the protocol can't force it: proofs can be fetched from an archive RPC (K1, G-COMM-2) |
| Relay transactions / blocks | ✅ (unrewarded) | Native p2p gossip. **Deliberately not rewarded**: relay work can't be verified ("no fake rewards") |
| Perform assigned verification tasks | ✅ | ICS-23 state proofs at derived (store, height, key); `community_test.go`, e2e |
| Data availability services | ❌→v2 | Not implemented (Q4). Counted under §9 |
| Disconnect / reconnect, keep identity and config | ✅ | identity = operator key + on-chain node; onion file local; offline never penalised; `TestOfflineNodeNoPenaltyAndRejoin` |
| Not required to be validators | ✅ | separate module, separate bond |
| Different hardware / connectivity | ✅ | no uptime requirement; pruned node is enough (window + epoch_blocks versions) |
| Fake identities can't farm rewards | ✅ | selection ∝ bond, min bond per identity; `TestSelectionProportionalToBond` (splitting gains nothing) |
| Not trusted on claims; work verifiable | ✅ | every paid unit has an on-chain-verified proof (INV-COMM-2) |

## §2.3 Full nodes, archive nodes, sync

| Requirement | Verdict | Evidence |
|---|---|---|
| Full nodes validate independently | ✅ | default node mode |
| Archive nodes | 🟢 | `pruning = "nothing"`; not tested (G-OPS-2) |
| Join via verifiable snapshot | 🟢 | CometBFT state sync verifies snapshots against light-client headers; not configured or tested (G-OPS-2) |
| No dependence on centralised RPC | 🟡 | P2P sync needs no RPC; state sync needs ≥ 2 RPC servers for light-client checks, and the trusted height/hash procedure isn't defined (G-CONS-3) |

## §3 Consensus and finality

| Requirement | Verdict | Evidence |
|---|---|---|
| Stake-weighted BFT, safety first | ✅ | CometBFT; < 1/3 Byzantine assumption documented (architecture T1–T3) |
| Proposal selection / validation, prevote, precommit, commit | 🟢 | CometBFT round logic (weighted round-robin proposer) |
| Voting-power calculation | ✅ | equal genesis power verified on the 4-node net |
| Validator-set updates | 🟢 | EndBlock updates, applied at H+2 |
| Timeouts | 🟢 | config.toml consensus timeouts (`timeout_commit` 1 s on test nets) |
| Partition handling | 🟡 | Halt-not-fork shown by stopping 2/4; a real network partition is not simulated |
| Downtime handling | ✅ | 1/4 down stays live (`check_localtest.sh`) |
| Halt rather than conflict | ✅ | 2/4 down halts; identical hashes after recovery |
| Double-sign protection / recovery procedures | 🟢 | evidence handling built in; recovery runbook not written |
| Community nodes can't affect consensus | ✅ | x/community emits no validator updates and hooks never error (INV-COMM-8); chain runs with zero community nodes |
| No custom consensus changes | ✅ | CometBFT unmodified |
| Predictable block times / efficient messaging | 🟢 | CometBFT; not benchmarked (G-TEST-4) |

## §4 Non-custodial staking and delegation

| Requirement | Verdict | Evidence |
|---|---|---|
| Self-bond, delegation, multiple and partial delegations | ✅ | `e2e.sh`, `evm-e2e` precompile delegate |
| Redelegation | 🟢 | `nirad tx staking redelegate` and precompile; confirmed path `BeginRedelegation → Unbond → Delegate`, so the cap applies. Not e2e tested (needs 2 validators) |
| Undelegation, unbonding period | ✅ | `e2e.sh` #5, `evm-e2e` precompile undelegate |
| Reward accumulation / withdrawal / commission | 🟢 | x/distribution F1, `withdraw-rewards`, commission ≥ 5 %; amounts not asserted (out of scope here) |
| Delegation tracking | ✅ | REST and precompile `delegation()` cross-checked |
| Slashing propagation | 🟢 | x/staking slashes delegators pro rata; not exercised |
| Validator eligibility | 🟢 | jailed/tombstoned excluded; min commission enforced |
| Protocol-held funds, no key custody | ✅ | bonded pool module accounts; validators never touch delegator keys |
| Inspect performance, commission, self-bond, total stake, slashing history | ✅ | `q staking validator`, `q slashing signing-info`, `q distribution slashes` (all present) |
| Bounded effective power per validator | ✅ | x/policy cap: unit tests INV-STAKE-CAP-1..3 (randomised); live rejection pending ≥ 15 validators (G-CONS-2) |
| Cap keeps economic accountability | ✅ | power stays = slashable stake (ADR-0004) |
| Cap can't trap funds | ✅ | undelegation never blocked (INV-STAKE-CAP-2) |
| Cap covers every path | ✅ | Cosmos tx, authz, redelegation and the EVM precompile all go through `Delegate` hooks (precompile tested live) |

## §5 Community verification layer

| Requirement | Verdict | Evidence |
|---|---|---|
| Random task assignment | ✅ | tasks derived from seed + operator (`DeriveTask`, spec vectors) |
| Unpredictable committee | ✅ | hash-onion: selection depends on a secret layer revealed only when claiming |
| No grinding | ✅ | layer bound to the epoch; rotation valid next epoch only (`TestINV_COMM_9_Reveal`, grinding test) |
| Independent block execution or verification | 🟡 | v1 proves **state possession** (Merkle proofs against committed app hashes), not re-execution. Re-execution / fraud proofs are v2 (ADR-0003) |
| State-root comparison | ✅ | divergence alerts compare a locally computed app hash with the committed one (`TestDivergenceAlerts`) |
| Signed reports | ✅ | every report is a signed tx (Cosmos or EVM) |
| Challenge mechanism | 🟡 | Proofs are self-verifying, so there's nothing to challenge; alerts are the challenge path for consensus results, but they're advisory |
| Dispute resolution | ✅ | alert threshold event → governance `MsgResolveAlert` (refund or burn) |
| Invalid report detection | ✅ | forged, tampered, copied and duplicate proofs rejected (`TestSubmitTaskProofRejections`, e2e) |
| Sybil resistance; no trust in majority of identities | ✅ | nothing decided by vote; payment ∝ verified units; selection ∝ bond |
| Verifiable completion / reward eligibility | ✅ | INV-COMM-1/2, settlement e2e on both interfaces |
| Penalties for provable misconduct | ✅ | A governance-rejected (false) alert forfeits the deposit **and** slashes 10 % of the bond (fixed G-COMM-1, `TestDivergenceAlerts`) |
| Doesn't slow consensus | ✅ | bounded hooks; proof checks gas-metered (incl. reveal, G-SEC-1) |
| Not decorative | ✅ | no reward without an on-chain-verified proof |

## §6 Native EVM

| Requirement | Verdict | Evidence |
|---|---|---|
| Ethereum txs (legacy, EIP-1559), signatures, nonces | ✅ | `evm-e2e`: type 0 and type 2, nonce increments |
| Account model / addresses | ✅ | eth_secp256k1; bech32 ↔ 0x same bytes (`e2e.sh` #1) |
| Bytecode execution, deploy, calls, storage | ✅ | Counter.sol deploy and calls |
| Events, logs, receipts | ✅ | logs parsed, `getLogs` with filter |
| Gas accounting / estimation | ✅ | estimateGas bounded; base fee present |
| Revert handling / custom errors | ✅ | `TooLarge` custom error decoded |
| Same consensus, blocks and finality | ✅ | cosmos/evm in the same state machine (ADR-0005) |
| Same state; native ↔ EVM balance consistency | ✅ | bank == `eth_getBalance` after Cosmos and EVM txs (both suites) |
| Staking balances from EVM | ✅ | staking precompile reads and writes |
| Native token in EVM | ✅ | NIRA is the EVM native currency (msg.value, 18 decimals); no wrapped duplicate |
| JSON-RPC: chain info, balances, submit, lookup, receipts, calls, logs | ✅ | e2e (13 methods) |
| JSON-RPC: event filters, pending tx filter, fee info, network status | 🟢 | probed live: `eth_newFilter`, `eth_newBlockFilter`, `eth_newPendingTransactionFilter`, `eth_feeHistory`, `eth_maxPriorityFeePerGas`, `eth_syncing`, `net_version/listening/peerCount` respond. Not in automated tests |
| Pending transactions | 🟡 | pending filter works; `txpool_*` is off by default (`api = "eth,net,web3"`); `eth_pendingTransactions` isn't implemented |
| WebSocket subscriptions | 🟢 | `eth_subscribe` on `:8546` (not over HTTP, as on Ethereum); not tested |
| Wallet chain identification | ✅ | `eth_chainId` and `net_version` = 47201, unused in the public registry |
| Common developer tools | 🟡 | ethers.js automated; Foundry, Hardhat and MetaMask not automated (G-TEST-3) |
| DoS / resource limits | 🟡 | block gas 100 M, EIP-1559; **fee floor ≈ 0 on default config** (K6), silent mempool drops (K7) |
| "Don't claim full compatibility without tests" | ✅ | docs claim only what's tested |

## §8 Validator selection and decentralisation

| Requirement | Verdict | Evidence |
|---|---|---|
| Self-bonded + delegated stake | ✅ | power = total bonded |
| **Performance, infrastructure reliability, slashing history in selection** | 🔶 | Decided (ADR-0008): power = slashable stake; performance gates **eligibility** (downtime jail, permanent tombstone, min self-bond, cap); history visible to delegators |
| Active-set capacity | ✅ | `max_validators` |
| Voting-power concentration | ✅ | x/policy cap |
| Stake monopolies / delegation concentration | ✅ | cap |
| Sybil validator identities / single-operator dominance | 🔶 | Not addressable on-chain without identity. Monitoring and disclosure only (K3) |
| Infrastructure concentration | ❌ | No protocol measure (K4). Monitoring is the realistic control |
| Validator collusion | 🟡 | BFT tolerates < 1/3; cap raises the number of colluders needed (≥ 4 to halt at 10 %) |
| Transparent path community → validator | ✅ | permissionless `create-validator`; community record (`units_total`) public |
| No reserved founder seats | ✅ | none in code; genesis builder treats every validator equally |
| Minimum self-bond as a qualification | ✅ | fixed (G-STAKE-1) |

## §9 Networking, §10 Storage

| Requirement | Verdict | Evidence |
|---|---|---|
| Discovery, authenticated encrypted connections | 🟢 | CometBFT PEX + SecretConnection (node keys) |
| Gossip, block propagation, sync | ✅ | multi-node net works |
| Peer reputation / malicious-peer protection | 🟡 | CometBFT marks bad peers in the address book; no tuned policy (G-SPEC-4) |
| Peer diversity, eclipse resistance | 🟡 | default inbound/outbound limits; no diversity rules (G-SPEC-4) |
| Connection limits, resource management | 🟢 | `max_num_inbound/outbound_peers`, mempool size config |
| Large network without all-to-all | 🟢 | gossip overlay; not measured |
| Recovery after long offline | 🟢 | block sync / state sync; not tested |
| Snapshot creation, verification | 🟢 | SDK snapshots; not configured (G-OPS-2) |
| Pruning, archive, indexing, historical queries | 🟢 | pruning modes; CometBFT kv indexer; historical queries at any retained height (community proofs depend on this, and are tested) |
| Defined storage per node class | 🟡 | documented in OPERATIONS.md; not measured |
| Data availability layer | ❌ | v2 (Q4) |

## §12 Governance

| Requirement | Verdict | Evidence |
|---|---|---|
| Proposals, deposits, voting, periods, quorum, thresholds | 🟢 | x/gov; params in genesis network file |
| Parameter changes (incl. x/policy, x/community) | 🟢 | module authority = gov; `MsgUpdateParams` gov-only (unit-tested authority checks) |
| Treasury | 🟢 | community pool spend proposals |
| Upgrades | 🟡 | x/upgrade present; **no upgrade-handler framework** (G-OPS-1) |
| Validator and economic policy changes | 🟢 | via params |
| No hidden admin powers | ✅ | every module authority = gov; circuit breaker has no authorised accounts (`account_permissions: []`) |
| Safeguards against governance attacks | 🟡 | quorum 33.4 %, veto 33.4 %, **25 % of min_deposit upfront** (fixed G-GOV-1); no formal governance attack analysis yet (G-SPEC-3) |

## §13 Node software

| Requirement | Verdict | Evidence |
|---|---|---|
| Operating modes (validator, community, full, archive) | ✅ | same binary, documented configs |
| Recovery after shutdown | ✅ | `run` loop stateless; restart-safe |
| Secure keys, nothing exposed remotely | 🟡 | local keyring; onion file 0600; no remote signer integration |
| Contribution and reward tracking | ✅ | `units_total`, `rewards_total`, `slashed_total` per node via CLI, REST and precompile `getNode` (e2e both interfaces) |
| Easy setup / cross-platform | 🟡 | CLI on Linux; Windows/macOS untested |
| Resource monitoring / limits | ❌ | none |
| Safe updates | ❌ | no Cosmovisor/upgrade flow (G-OPS-1) |
| Peer discovery, sync status | 🟡 | `nirad status`; no dedicated UI |

## New gaps from this review (added to GAPS.md)

| ID | Gap | Sev | Fix sketch |
|---|---|---|---|
| G-STAKE-1 | No protocol-wide minimum self-bond (§1A, §2.1, §8) | M | x/policy param `min_self_bond`; `AfterValidatorCreated` hook rejects `min_self_delegation < param` (covers Cosmos tx and the precompile). The SDK already jails a validator whose self-bond drops below its own `min_self_delegation`, and only lets it increase |
| G-GOV-1 | `min_initial_deposit_ratio = 0`: proposal spam with no upfront deposit | T | set e.g. 0.25 in network files; document |
| G-EVM-1 | `txpool` namespace off by default; JSON-RPC filters/fee/net methods not in automated tests | I | enable `txpool` on RPC nodes; add the probed methods to `evm-e2e` |
