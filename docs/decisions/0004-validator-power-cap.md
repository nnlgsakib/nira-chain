# ADR-0004: Per-validator voting-power cap enforced at delegation time

- Status: accepted (module `x/nirachain` renamed to `x/policy` by ADR-0007)
- Date: 2026-11-25
- Written by: design-protocol (protocol-staking-validators)
- Related: R-F3, R-F4, T7, K3

## Context
"A validator must not gain unlimited effective voting power simply by attracting unlimited delegation." Any cap must keep economic accountability: voting power must stay equal to slashable stake.

## Options considered
| Option | Pros | Cons |
|---|---|---|
| A. Clip voting power sent to consensus (power ≠ stake) | Hard cap on consensus weight | Breaks the "power = slashable stake" link; excess stake earns without risk-weighted voting; changes how CometBFT sees power |
| B. Reject delegations that push a validator above X % of total bonded stake | Power stays = stake; simple; deterministic; enforced in a staking hook so it covers Cosmos txs **and** the EVM staking precompile | Total-bonded changes (others unbonding) can leave a validator above the cap without new delegations; it can't grow further but isn't forced down |
| C. Quadratic / sub-linear rewards above a threshold | Economic nudge | Doesn't bound power |

## Decision
B. Parameter `P-STAKE-max_validator_power_bps` (genesis 1000 = 10 %, bounds 300–3333, governable). It is enforced in an `AfterDelegationModified` staking hook in `x/nirachain`, so every delegation path is covered (Cosmos tx, authz, EVM precompile, redelegation). The check is skipped while total bonded is below `P-STAKE-cap_activation_bonded`, so genesis bootstrapping with a few validators still works.

## Consequences
- With 10 % and BFT's 1/3 threshold, at least 4 independent validators must collude to halt the chain, and 7 to break safety.
- Doesn't stop one operator from running several validators (K3). That is addressed by monitoring and operator disclosure, not by protocol.

## Revisit when
Validator count > 50 (consider lowering to 5 %), or evidence of sybil validators.
