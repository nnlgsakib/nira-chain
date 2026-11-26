# Threat model — Nira Chain

- Version 0.1, 2026-11-25. Covers master prompt §11 (all 20 threats) for the code at this commit.
- Inputs: `docs/spec/architecture.md` (trust assumptions T1–T7), `docs/spec/community.md`, ADR-0002…0008, `docs/COMPLIANCE.md`, `docs/GAPS.md`.
- **Evidence column:** a test or script path means the protection is *demonstrated*. **PROPOSED** means it's designed or documented but not built. **ACCEPTED RISK** means it's recorded as a K-risk in CHAIN.md.

## 1. Assets

| Asset | Where | Worth attacking because |
|---|---|---|
| Bonded stake (validators + delegators) | x/staking bonded pool | slashing, theft by key compromise, consensus control |
| Community bonds, alert deposits, epoch reward budget | x/community module account | theft, fake work, reward drain |
| Community pool (protocol reserve) | x/distribution | governance capture |
| Consensus safety and liveness | CometBFT + active set | double spends, halts |
| EVM contract funds | x/vm state | contract exploits, precompile bugs |
| Governance authority | x/gov (authority of every module) | changes any parameter or code (upgrade) |
| Validator consensus keys, operator keys | operators' machines / signers | double sign, theft of self-bond |
| Release pipeline, genesis file | GitHub, coordinator | malicious binary or genesis = total compromise |

## 2. Trust boundaries

1. **Consensus boundary:** < 1/3 Byzantine voting power (T1), > 2/3 online for liveness (T2).
2. **State machine ↔ EVM:** precompiles (`0x…0800–0806`, `0x…0900`) are consensus code reachable by any contract.
3. **State machine ↔ community nodes:** every community message is untrusted input. Verification is cryptographic (ICS-23), never by vote.
4. **Node ↔ peers:** P2P is untrusted. Full validation of every block.
5. **Clients ↔ RPC:** JSON-RPC, REST and gRPC are API surface, not consensus. Validators don't expose them.
6. **Humans ↔ keys:** operators, the genesis coordinator, release signers.

## 3. Threats

Format per threat: **Attack · Subsystem · Prevention · Detection · Recovery · Evidence**.

### TM-01 Byzantine validators (< 1/3)
- **Attack:** validators equivocate, censor, or propose invalid blocks.
- **Subsystem:** consensus.
- **Prevention (implemented):** CometBFT, unmodified (ADR-0002). Safety holds with < 1/3 Byzantine. Invalid blocks are rejected by the execution of every honest node.
- **Detection:** equivocation becomes CometBFT evidence; missed votes appear in x/slashing signing info.
- **Recovery:** automatic slashing and jailing (see TM-03); a censoring proposer is skipped in later rounds.
- **Evidence:** `tests/genesis/check_localtest.sh` (1/4 down stays live); `tests/consensus/check_validators.sh` (equivocation slashed, no fork).

### TM-02 Validator collusion (≥ 1/3 or ≥ 2/3)
- **Attack:** ≥ 1/3 halt the chain; ≥ 2/3 can finalise anything they sign, including double spends via a fork.
- **Subsystem:** consensus, staking.
- **Prevention:**
  - *Implemented:* per-validator power cap of 10 % (ADR-0004), so ≥ 4 independent validators are needed to halt and ≥ 7 to break safety; the genesis builder rejects any validator ≥ 1/3; min self-bond (G-STAKE-1) puts operator capital at stake.
  - *Proposed:* public operator-identity disclosure and infrastructure-diversity monitoring (K3, K4).
- **Detection:** a fork produces conflicting commits; equivocation evidence is slashable while within the evidence age. Halt = no blocks.
- **Recovery:** halt is the safe outcome (no conflicting finality, `check_localtest.sh`). A fork needs social coordination plus slashing of all equivocators; the runbook is PROPOSED (`docs/security/runbooks.md` to be written).
- **Evidence:** cap live rejection (`check_validators.sh` #2); 2/4 down halts without forking (`check_localtest.sh`).
- **Residual:** one entity running many validators (sybil validators) is not detectable on-chain. **ACCEPTED RISK K3.**

### TM-03 Double signing
- **Attack:** a validator (or a misconfigured duplicate node) signs two different votes at the same height and round.
- **Subsystem:** consensus, slashing.
- **Prevention (implemented):** x/evidence + x/slashing: 5 % of infraction-height stake burned, permanent tombstone, delegators slashed pro rata. Operator docs forbid running two signers with one key.
- **Prevention (proposed):** remote signer with double-sign protection (TMKMS/Horcrux) for mainnet validators (key inventory).
- **Detection:** CometBFT evidence → `slash` event, `tombstoned = true` in signing info.
- **Recovery:** automatic. The tombstoned key can never validate again; delegators redelegate.
- **Evidence:** `tests/consensus/check_validators.sh` #3, using real `DuplicateVoteEvidence` from `tools/doublesign`. Asserts jailed, tombstoned, exact 5 % burn, pro-rata delegator loss, no fork.

### TM-04 Long-range attacks
- **Attack:** keys of validators who have fully unbonded are used to build an alternative history from an old height, then shown to new or long-offline nodes.
- **Subsystem:** sync, light clients.
- **Prevention:**
  - *Implemented:* unbonding 21 days, longer than the evidence max age (2 days / 100 000 blocks); state sync requires a trusted height and hash and ≥ 2 RPC servers for light-client verification.
  - *Implemented:* `scripts/trusted-checkpoint.py` takes a checkpoint only when ≥ 2 independent servers agree on the block hash and validator set (refuses a forging server, tested); procedure in `runbooks.md` RB-6.
- **Detection:** a light client checking against multiple providers sees conflicting headers (CometBFT light-client attack detection).
- **Recovery:** social. Operators use the checkpoint from independent sources.
- **Evidence:** `tests/sync/check_sync.sh` (state sync from a light-client-verified snapshot). The weak-subjectivity procedure is PROPOSED.

### TM-05 Stake concentration
- **Attack:** a whale or exchange accumulates delegations into one validator and dominates consensus or governance.
- **Subsystem:** staking.
- **Prevention (implemented):** power cap at delegation time on every path (Cosmos tx, authz, redelegation, EVM precompile); undelegation is never blocked; activation thresholds avoid bootstrap deadlock.
- **Detection:** `nirad q staking validators` power shares; the genesis report's Nakamoto coefficient.
- **Recovery:** governance can tighten `max_validator_power_bps` (bounds 3–33 %).
- **Evidence:** `x/policy/keeper/power_cap_test.go` (INV-STAKE-CAP-1..3, 2 000 randomised operations); live rejection via Cosmos and EVM in `check_validators.sh` #2.
- **Residual:** governance voting power isn't capped (stake-weighted); see TM-19.

### TM-06 Sybil attacks
- **Attack (validators):** one operator registers many validators. Partially mitigated by the min self-bond (100 000 NIRA each) and the cap. **ACCEPTED RISK K3.**
- **Attack (community nodes):** split one bond into many identities to farm selections or rewards.
- **Subsystem:** x/community.
- **Prevention (implemented):** selection probability and rewards ∝ bond; min bond 1 000 NIRA per identity; payment only per verified unit. Splitting gives the same expectation with more capital locked.
- **Detection:** not needed for neutrality. Node registry is public (`q community nodes`).
- **Evidence:** `TestSelectionProportionalToBond` (splitting gains nothing); INV-COMM-2.

### TM-07 Eclipse attacks
- **Attack:** an attacker monopolises a node's peers to feed it a stale or fake chain view.
- **Subsystem:** P2P.
- **Prevention (implemented):** every block is fully validated (a fake chain needs > 2/3 validator signatures); persistent peers and PEX.
- **Prevention (proposed):** sentry topology for validators, peer limits and diversity rules, multiple seeds (G-SPEC-4).
- **Detection:** `catching_up` / height lag versus public explorers; `net_info` peer list.
- **Recovery:** add trusted persistent peers; state sync with an independent trust hash.
- **Evidence:** none specific. **ACCEPTED RISK** until G-SPEC-4.

### TM-08 Network partitions
- **Attack / fault:** the network splits.
- **Subsystem:** consensus.
- **Prevention (implemented):** a BFT side with ≤ 2/3 power cannot commit. The chain halts rather than forking (safety over liveness).
- **Detection:** no new blocks; peer counts drop.
- **Recovery:** automatic when connectivity returns. Nodes catch up by block sync.
- **Evidence:** `check_localtest.sh` (2/4 stopped: halt, no fork, identical hashes after recovery). A real network partition (as opposed to stopped nodes) is not yet simulated: G-CONS-1.

### TM-09 Denial of service
| Vector | Prevention (status) | Evidence |
|---|---|---|
| Tx spam | EIP-1559 base fee + **min gas price floor** (template 10 gwei; K6 needs operators' `minimum-gas-prices`); mempool cap 4 096 (implemented) | `docs/benchmarks` (rejections at cap) |
| Block execution DoS | finite block gas 100 M; every custom operation gas-metered: proof bytes, **onion reveal hashing (G-SEC-1, fixed)** | `TestRevealIsGasMetered` |
| Unbounded BeginBlock/EndBlock | x/community work bounded by `max_proofs_per_epoch`, `max_unbond_per_block`, ≤ 2 ring deletions per block; hooks never error (INV-COMM-8) | `TestHookFailureIsolated`, `TestINV_COMM_Randomised` |
| State bloat | community bond and deposit required; app-hash ring bounded (INV-COMM-7) | `checkInvariants` |
| RPC abuse | APIs off on validators; RPC nodes behind proxy and rate limits (PROPOSED: ops-infrastructure) | — |
| Malformed inputs | fuzzed decoder and handlers (G-SEC-3) | `make fuzz` (found and fixed a zero-window panic) |

### TM-10 Malicious peers
- **Attack:** peers send invalid blocks or txs, flood messages, or misbehave.
- **Subsystem:** P2P.
- **Prevention (implemented):** CometBFT authenticated encrypted connections (SecretConnection), message size limits, ban on protocol violations, full validation.
- **Prevention (proposed):** tuned connection limits and a sentry layer (G-SPEC-4).
- **Evidence:** CometBFT upstream; no Nira-specific test.

### TM-11 Invalid blocks
- **Attack:** a proposer proposes a block with invalid txs or wrong state.
- **Subsystem:** consensus / execution.
- **Prevention (implemented):** every validator executes the block (ProcessProposal / FinalizeBlock); app-hash mismatch makes honest nodes refuse. Determinism rules (no floats, no wall clock, no map iteration) are audited.
- **Detection:** `CONSENSUS FAILURE` / app-hash mismatch logs; community divergence alerts (advisory).
- **Recovery:** halt and investigate; the faulty binary is fixed by upgrade.
- **Evidence:** determinism audit (0 findings); identical block hashes across 4 and 16 nodes in all multi-node tests.

### TM-12 Invalid state transitions (logic bugs)
- **Attack:** exploit a bug in x/community, x/policy or SDK modules to mint, steal or skip checks.
- **Subsystem:** modules.
- **Prevention (implemented):** every check C1–C12 tested including rejections; invariants INV-COMM-1..10 and INV-STAKE-* checked every block in randomised tests; fuzzing; least-privilege module account (community: no mint/burn/staking); gov-only authorities.
- **Prevention (proposed):** external audit (G-SEC-8); formal model of settlement (Q5).
- **Detection:** invariant violations; `comm_hook_error` events; supply monitoring (PROPOSED).
- **Evidence:** `x/community/keeper/community_test.go`, `fuzz_test.go`, `power_cap_test.go`.

### TM-13 EVM execution attacks
| Vector | Prevention (status) | Evidence |
|---|---|---|
| Reentrancy into precompiles / partial writes | cosmos/evm cache-context per precompile call; fix of GO-2025-3684 confirmed present in v0.6.0 | `tests/evm-e2e/atomicity.mjs`: caught inner revert after registerNode + delegate leaves no state |
| Precompile used to bypass module rules | precompiles call the same msg server and hooks (cap, min self-bond) | `e2e.mjs` precompile delegate under cap, createValidator below floor reverts |
| Replay across chains | EIP-155 chain IDs 91000 / 92000 / 93000 per network (from genesis; a node with a conflicting app.toml value refuses to start); `allow-unprotected-txs = false` | `e2e.mjs` chainId check |
| Balance inconsistency native ↔ EVM | single bank state; balance handler for precompile coin moves | bank == `eth_getBalance` checks in all suites |
| Contract-level bugs | out of protocol scope (application security) | — |

### TM-14 Gas manipulation
- **Attack:** underpriced operations (cheap CPU), gas refund tricks, fee-market gaming.
- **Subsystem:** EVM, x/community, x/feemarket.
- **Prevention (implemented):**
  - every custom operation is metered: proof bytes at 20 gas/byte, reveal at 72 gas/hash, base KV costs;
  - EIP-1559 base fee with a min-gas-price floor;
  - EVM opcode pricing from geth (cosmos fork).
- **Evidence:** `TestRevealIsGasMetered`; benchmark: `SubmitTaskProof` ≈ 36.6k gas for ≈ 45 µs.
- **Residual:** base fee decays to ~0 without a floor. **K6:** the template sets 10 gwei; operators must set `minimum-gas-prices`.

### TM-15 Reward exploitation
| Vector | Prevention (status) | Evidence |
|---|---|---|
| Paying for unverified work | pay only per TaskDone record from a passed proof (INV-COMM-2) | `TestSubmitTaskProofRejections` |
| Double pay | TaskDone per (epoch, operator, index) | duplicate rejected in unit + both e2e |
| Overpay the budget | floor division; remainder to community pool (INV-COMM-1) | `TestProofAndSettlement` |
| Copying another node's proof | tasks are derived from the operator address | copy test (`ErrWrongTask`) |
| Outsourcing proofs to an archive RPC | not preventable without GPU-class proof of replication. **ACCEPTED RISK K1** (ADR-0009): rewards are defined as paying for correct, live state access, not storage | — |

### TM-16 Fake community work
- **Attack:** submit fabricated proofs, forged values, or random bytes.
- **Subsystem:** x/community.
- **Prevention (implemented):** ICS-23 verification against the committed app hash of the exact (store, height, key) derived for that operator; fuzzed verifier never accepts random input.
- **Evidence:** `FuzzVerifyStateProof` (random proofs never verify), `FuzzSubmitTaskProof`, tampered-proof tests in unit and both e2e.

### TM-17 Random committee manipulation
- **Attack:** grind selection (try many secrets), predict others' selection to target them, or bias the seed.
- **Subsystem:** x/community randomness.
- **Prevention (implemented):**
  - hash-onion layers fixed at commitment and bound to the epoch number, so a node can't grind;
  - rotation is valid only from the next epoch, whose seed is unknown;
  - selection depends on a secret revealed only when claiming, so others can't predict it;
  - distance-bounded and gas-metered.
- **Residual:** a proposer can bias the per-epoch seed via block contents; the impact is reward fairness only, never finality. **ACCEPTED RISK K2.**
- **Evidence:** `TestINV_COMM_9_Reveal` (grinding layer rejected), `FuzzVerifyReveal` (soundness), spec test vectors.

### TM-18 Key theft
- **Attack:** steal a validator consensus key (double-sign, slash), an operator key (steal self-bond, change commission), a user key, or a community onion secret (predict selections).
- **Subsystem:** operations.
- **Prevention:**
  - *Implemented:* no protocol admin keys (all authorities = gov; circuit breaker has no authorised accounts); onion file 0600 and never transmitted; dev keys documented as test-only.
  - *Proposed:* remote signer / HSM for validators, hardware wallets for operators (see `key-inventory.md`).
- **Detection:** unexpected signing-info changes, commission edits, equivocation; alert monitors PROPOSED.
- **Recovery:** compromised operator key → unbond and recreate; stolen consensus key → tombstoned on misuse, rotate (SDK 0.55 adds key rotation); stolen onion → `rotate-onion`.

### TM-19 Governance attacks
| Vector | Prevention (status) | Evidence |
|---|---|---|
| Proposal spam | min deposit 10 000 NIRA, **25 % upfront** (G-GOV-1) | genesis params; `governance.sh` |
| Low-turnout capture | quorum 33.4 %, threshold 50 %, veto 33.4 % (implemented) | — |
| Flash-stake voting | voting power = bonded stake, with 21-day unbonding (implemented) | — |
| Malicious upgrade | the upgrade only takes effect if validators run the new binary; old binaries halt at the height (implemented) | `tests/upgrade/rehearsal.sh` |
| Malicious parameter change | every parameter has validated bounds (cap 3–33 %, community params, min self-bond) | param validation tests |
| Treasury drain | community pool spends need a passing proposal | — |
| Timelock / emergency council | PROPOSED (G-SPEC-3) | — |

### TM-20 Economic attacks
- **Attack:** buy > 1/3 stake to halt, > 2/3 to control; bribery; drive the token price down to cheapen an attack; farm community rewards at a loss to the protocol.
- **Subsystem:** tokenomics, staking.
- **Prevention:**
  - *Implemented:* slashing makes equivocation costly; cap raises the coordination needed.
  - *Proposed:* security-budget model and simulations (G-SPEC-2, excluded from the current scope by the project owner).
- **Evidence:** none yet. **ACCEPTED RISK** until the tokenomics work (Q1/Q3).

## 4. Summary

| Status | Threats |
|---|---|
| Prevention implemented **and demonstrated by tests** | TM-01, 03, 05, 06 (community), 08, 09 (most vectors), 11, 12, 13, 14, 15 (except K1), 16, 17, 19 (most) |
| Partially implemented / upstream-only | TM-02, 04, 07, 10, 18 |
| Open (accepted risks) | K1 proof outsourcing, K2 seed bias, K3 sybil validators, K4 infrastructure concentration, K6 fee floor (config), TM-20 economics |

## 5. Next security work
1. ~~Runbooks~~ done: `docs/security/runbooks.md` (RB-1…RB-8).
2. Sentry and P2P hardening spec (G-SPEC-4) to close TM-07 and TM-10.
3. On-chain monitors and alerts: equivocation, voting-power moves, supply, governance proposals, `comm_hook_error` (G-OPS-3).
4. External audit of x/community, x/policy, the precompile and the genesis builder (G-SEC-8), then a bug bounty before value arrives.
