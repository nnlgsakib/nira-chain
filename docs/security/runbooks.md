# Incident runbooks

Version 0.1, 2026-11-26. For validator operators, the core team on call, and governance participants.

**Detection** comes from:
- `scripts/monitor.py`, which emits JSON alerts;
- the Prometheus rules in `deploy/monitoring/alerts.yml`;
- operator reports.

**Severity levels:**
- **S1:** chain halted, fork, funds at risk.
- **S2:** degraded but finalising (validators jailed, high block time, community hook errors).
- **S3:** informational.

**First response for any S1** (within 15 minutes):
1. Open the incident channel and name an incident lead.
2. Freeze non-essential changes: no upgrades and no parameter proposals.
3. Collect evidence before restarting anything: logs, `nirad status`, `/dump_consensus_state`, the last 3 blocks from several nodes.
4. If funds are actively being drained through an application bug, contact SEAL 911 (security alliance), and publish the bug-bounty / Safe Harbor policy once it exists.

---

## RB-1 Chain halted (no new blocks)

**Alerts:** `halt` (monitor), `ChainHalted`, `ValidatorsMissingVotes` (Prometheus).

| Cause | How to tell | Action |
|---|---|---|
| ≥ 1/3 of voting power offline | `missing_validators_power / validators_power ≥ 0.33`; peers report the same height | Contact the missing validators (operator contact list). Each restarts its node, and the chain resumes by itself once > 2/3 sign. No data loss, no fork (tested: `check_localtest.sh`, Docker partition test) |
| Planned upgrade height reached | log line `UPGRADE "<name>" NEEDED at height: H` on every node | Expected. Operators switch to the release binary (RB-4) |
| Network partition | groups of nodes at the same height, each with peers only on its own side | Restore connectivity (provider/BGP incident, firewall). The chain resumes automatically; the minority catches up by block sync |
| Consensus failure / app-hash mismatch | `CONSENSUS FAILURE` or `wrong Block.Header.AppHash` in logs on some nodes | **Do not restart blindly.** This is non-determinism or a bug: compare `nirad version` across nodes (mixed binaries is the usual cause); gather `--trace` output; go to RB-5 |

**Never:** run `unsafe-reset-all` on a validator, or start a second copy of a validator to "help". That causes double signing (RB-2).

## RB-2 Double signing or a suspected fork

**Alerts:** `slash` with reason `double_sign`, `ByzantineValidatorsSeen`, `jailed`; two nodes report different hashes for the same height.

1. **Self-inflicted** (an operator ran the same key on two machines): stop the duplicate immediately. The validator is tombstoned permanently; that can't be undone. The operator creates a new validator with a new key, and delegators redelegate (instantly, still subject to the 21-day slashing window).
2. **Fork** (two committed blocks at one height): this requires ≥ 1/3 Byzantine power, so it's an S1 attack.
   - Stop all RPC nodes from serving the minority fork.
   - Identify both commits: `/commit?height=H` from nodes on each side. The signatures prove who equivocated.
   - Submit the evidence. CometBFT does this automatically when it sees both votes; `tools/doublesign` shows the evidence format.
   - Coordinate the canonical chain socially: validators agree on the fork that keeps > 2/3 of honest power, and publish a trusted checkpoint (RB-6).
3. Post-mortem within 7 days.

## RB-3 Key compromise

| Key | Immediate action | Follow-up |
|---|---|---|
| Validator consensus key | Stop the validator, so the attacker can't double-sign with your stake while you also sign. If the attacker double-signs first, the validator is tombstoned | New validator with a new key in a remote signer; announce it to delegators |
| Validator operator key | Move the self-bond: `unbond` everything (21 days, still slashable). Change the commission and withdraw address before the attacker does | New operator key, new validator |
| Community node operator key | Deregister the node (the bond comes back after the community unbonding period, 100 800 blocks) | Register a new node with a new key and a new onion |
| Community onion secret | `tx community onion-init --overwrite` + `rotate-onion` (valid from the next epoch). Impact is limited: others could predict that node's selections | — |
| Genesis/treasury allocation key | Move the liquid funds to a new hardware wallet. Vesting accounts can't move their locked portion, but the attacker can't either; the locked portion unlocks only to that account | Rotate the multisig signers |
| Release signing key | Revoke it, publish a new key out of band, re-sign the current release | Audit CI for other leaks |

## RB-4 Upgrade incident

| Situation | Action |
|---|---|
| Normal upgrade | At height H the old binary halts with `UPGRADE NEEDED`. Install the release, verify `SHA256SUMS`, start it. The handler runs once and blocks continue (rehearsed: `tests/upgrade/rehearsal.sh`) |
| New binary panics in the upgrade handler | Do **not** start the old binary past H (it refuses anyway). Fix the handler in a patch release with the same plan name; every operator installs it and starts. State before H is untouched |
| Plan must be cancelled before H | Governance `MsgCancelUpgrade` (expedited proposal). If the proposal can't pass in time, every operator starts with `--unsafe-skip-upgrades H`, which needs > 2/3 coordination |
| Some operators upgraded too early | Their node errors with `BINARY UPDATED BEFORE TRIGGER` and stops; it doesn't fork. Reinstall the previous binary until H |

## RB-5 Non-determinism (app-hash mismatch)

1. Find which nodes disagree, and the first height where the app hash differs (`/block?height=…` `app_hash` from several nodes).
2. Compare binaries (`nirad version --long`), Go versions and build flags. Reproducible builds (`scripts/verify-reproducible.sh`) let anyone check them.
3. Export state at height H−1 from a node on each side (`nirad export --height H-1`) and diff the module states to locate the module.
4. Community nodes may already have filed **divergence alerts** for that height (`divergence_threshold` event). Their reports help find the faulty side.
5. Fix with a patch release and coordinated restart. If the majority side is wrong, coordinate a rollback with `nirad rollback` on every node, which must be agreed by validators holding > 2/3.

## RB-6 Publishing a trusted checkpoint (weak subjectivity)

Needed for state-sync joins and nodes offline longer than the 21-day unbonding period.

```bash
scripts/trusted-checkpoint.py --rpc https://rpc.operator-a.example --rpc https://rpc.operator-b.example \
  --rpc https://rpc.operator-c.example --offset 1000
```

The tool requires every server to return the same block hash and validator set at the chosen height. If they disagree it exits with code 2 and prints **DO NOT SYNC** (tested against a forging server). Publish the agreed `trust_height` / `trust_hash` through ≥ 2 independent channels: documentation site, operator announcements, social. Node operators use `--write ~/.nira/config/config.toml` to apply it.

## RB-7 Community layer incidents (never consensus-critical)

| Alert | Meaning | Action |
|---|---|---|
| `comm_hook_error` | An x/community block step failed. The step's state is discarded, the chain continues (INV-COMM-8), and funds stay in the module account | Collect the error attribute and file a bug. The step retries next block; if it keeps failing, governance can set `reward_share_bps = 0` to stop funding while it's fixed |
| `divergence_threshold` | Community nodes with ≥ 1/3 of community bond report a different app hash at a height | Treat as a possible RB-5. Governance resolves each alert (`MsgResolveAlert`): valid → refund; false → deposit + 10 % bond slashed |
| No community proofs for many epochs | Unpaid budgets go to the community pool, so no harm to the chain | Check `nirad tx community status` on reference community nodes (pruning, onion exhausted, gas price) |

## RB-8 Spam / fee-market abuse

**Signals:** `MempoolNearlyFull`, rising block time, many rejections at the RPC edge.

1. RPC operators: tighten proxy rate limits and JSON-RPC caps (`node-profile.py rpc`).
2. Validators: raise `minimum-gas-prices` in app.toml. This is local, takes effect on restart, and needs no governance.
3. If it persists: governance raises `feemarket.min_gas_price` (expedited proposal).

---

**Rehearsal schedule:** RB-1, RB-2 and RB-4 at least twice a year on the Docker testnet (`make testnet-up`; `testnet.py test partition`; `make upgrade-rehearsal`). Record the results in the incident log.
