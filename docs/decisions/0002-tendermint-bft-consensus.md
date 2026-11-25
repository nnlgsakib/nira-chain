# ADR-0002: Tendermint-family BFT consensus with a bounded active set

- Status: accepted
- Date: 2026-11-25
- Written by: design-protocol (protocol-consensus)
- Related: R-N1, R-N2, R-N3, T1–T3

## Context
The idea asks for propose / prevote / precommit voting, single-block finality, halting instead of forking, and no custom consensus without formal review. It also asks for a manageable validator set that can grow.

## Options considered
| Option | Pros | Cons |
|---|---|---|
| A. Tendermint (CometBFT) | Exactly the propose/prevote/precommit model required; instant finality; halts on < 2/3; 7+ years in production; evidence handling built in | O(n²) votes: practical limit is about 150–200 validators |
| B. HotStuff family | Linear messages, larger sets | No production-grade Go engine integrated with an EVM state machine that we can adopt without writing consensus code |
| C. Custom consensus | Tailored to us | Violates "no novel consensus without formal review"; high risk |

## Decision
A. Use the unmodified CometBFT engine. The active set is capped by a governable parameter (genesis 100 max) and selected deterministically as the top-N by bonded power among non-jailed validators. **No consensus code is modified.**

## Consequences
- Safety holds with < 1/3 Byzantine power. The chain halts with > 1/3 offline (stated, intended).
- Block time comes from CometBFT timeouts (`timeout_propose`, `timeout_commit`); target ≤ 2 s p50 (proposed, must be benchmarked).
- Growing past about 150 validators would need a different engine. The participation layer is designed so that "more decentralisation" happens in community nodes, not in a larger BFT set.

## Revisit when
Benchmarks show block time > 4 s p95 at 100 validators, or demand for > 150 validators.
