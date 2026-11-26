# ADR-0003: Community layer = hash-onion sortition + ICS-23 state-possession proofs

- Status: accepted (v2 task plan superseded by ADR-0009: no storage-proof tasks for consumer nodes)
- Date: 2026-11-25
- Written by: design-protocol (protocol-verifiable-participation, protocol-randomness)
- Related: R-F5–R-F8, INV-COMM-*, K1, K2

## Context
Community Nodes must do work that is (a) useful, (b) verifiable on-chain without trusting the reporter, (c) not copyable from other nodes, (d) not on the finality path, and (e) tolerant of home nodes going offline. Simply "reporting the app hash" is worthless: the app hash is already public in the next header, so it can be copied.

## Options considered
| Option | Verifiability rung | Pros | Cons |
|---|---|---|---|
| A. Signed "block is valid" reports, majority vote | 4 (witness) | Simple | Lazy copying, sybil majority, decorative |
| B. Re-execution fraud proofs (optimistic) | 2 | Strongest semantic check | Needs a one-step EVM+SDK fraud-proof machine; very large build |
| C. State-possession proofs: random key at random recent height, ICS-23 proof against committed app hash | 1 (crypto proof) | Verified on-chain in O(log n), no trust in the reporter, impossible to fake without the state | Proves *access* to state, not independent execution; outsourcing to a public archive RPC is possible (K1) |
| D. C + divergence alerts (advisory) + later DA sampling / history serving | 1 + advisory | Usable now, extensible | Alerts are not enforceable by themselves |

Randomness for selection:
| Option | Pros | Cons |
|---|---|---|
| Block hash only | Free | Proposer can grind |
| Threshold BLS beacon via vote extensions | Unbiasable | Large build (DKG), touches consensus path |
| Per-node hash onion (commit `H^K(s)` at registration, reveal one layer per epoch) mixed with the finalised block hash | The node can't grind (layers fixed in advance); others can't predict its selection; pure SHA-256, verifiable | A proposer can still bias the block-hash part once per epoch (T6). Bounded impact: reward fairness only |

## Decision
D with hash-onion sortition. v1 task = state-possession proof. Divergence alerts are bonded and advisory. DA sampling and history-serving tasks are planned as v2 task types behind the same assignment → proof → settle interface.

## Consequences
- Every paid unit has an on-chain verification record (INV-COMM-2). No self-reported metric is ever paid.
- Sybil splitting gains nothing: selection probability and rewards are proportional to bond, with a minimum bond per identity.
- Offline = no reward plus decaying miss counter, never a slash. Slashing only for provable fraud (an invalid onion reveal is rejected; double registration of the same onion is slashable).
- K1 (outsourcing to an RPC) is an explicit, documented limitation. Mitigations: short proof deadline (P-COMM-proof_window), heights sampled from the window, and v2 tasks that require local data (history chunks, DA samples).

## Revisit when
v2 task types are specced, or measurements show most proofs come from a few RPC endpoints.
