# ADR-0008: Validator-set cadence and performance-aware selection

- Status: accepted (owner may override: CHAIN.md Q8/Q9)
- Date: 2026-11-25
- Written by: protocol-staking-validators / protocol-consensus
- Related: idea §2.1 ("epoch-based validator management"), §8 ("consider performance, reliability, slashing history"), G-DES-1, G-DES-2, ADR-0002, ADR-0004

## Context
The master prompt asks for epoch-based validator management and for selection that considers performance and slashing history. x/staking recomputes the active set every block (applied by CometBFT at H+2) and ranks purely by bonded stake. Performance acts through x/slashing: downtime jails the validator, and double-signing tombstones it permanently.

## Options considered

### Cadence
| Option | Pros | Cons |
|---|---|---|
| A. Per-block updates (current) | A jailed, tombstoned or slashed validator loses power **within 2 blocks**; no window where a misbehaving validator keeps voting; battle-tested | Set can change at any block (in practice it changes only on stake moves, jailing or unbonding) |
| B. Epoch batching (apply changes every N blocks) | Predictable set; fewer updates | A Byzantine or compromised validator keeps its voting power until the epoch ends; needs custom staking EndBlock and evidence/jail interaction code: consensus-critical novelty |
| C. Hybrid: epoch batching for joins and stake changes, immediate removal for jail/tombstone | Predictability for growth, safety for removals | Still custom consensus-critical code; diverges from x/staking invariants and its well-audited paths |

### Selection
| Option | Pros | Cons |
|---|---|---|
| D. Power = stake; performance acts through **eligibility** (downtime jail, double-sign tombstone, min self-bond, power cap) | Power always equals slashable stake (economic accountability); on-chain, deterministic, unforgeable signals | Uptime doesn't raise power |
| E. Weight power by an uptime score | Rewards reliability in voting power | Power ≠ slashable stake (attack cost drops); score windows gameable; custom consensus-weight code |
| F. Rank admission by score | Same | Same, plus "who is in the set" depends on a derived metric that nodes must agree on exactly |

## Decision
- **A (per-block updates).** The master prompt ranks safety first (§3). Per-block updates remove misbehaving validators fastest, and the only cost is predictability, which isn't a safety property. "Epoch-based management" is met where it adds value without safety cost: the community layer runs on epochs, rewards accrue per epoch, and governance changes `max_validators` deliberately.
- **D (stake-weighted power; performance gates eligibility).** Concretely, on chain and deterministic:
  - downtime: jailed when signed < 5 % of 10 000 blocks; must unjail after 600 s;
  - double-sign: 5 % slash plus a **permanent** tombstone;
  - entry: `min_self_delegation ≥ min_self_bond` (G-STAKE-1);
  - concentration: per-validator cap (ADR-0004).

  Delegators see performance and slashing history through `q slashing signing-info` and `q distribution slashes`, and can act on it by redelegating.

## Consequences
- No custom consensus-weight or staking-EndBlock code; x/staking and x/slashing invariants stay intact.
- Reliability is rewarded economically (jailed validators earn nothing and lose delegations), not by extra power.
- Revisit if the set grows past about 100 validators and per-block churn measurably affects block time (benchmark G-TEST-4), or if governance wants epoch batching for joins (option C).
