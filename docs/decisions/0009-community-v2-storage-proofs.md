# ADR-0009: Community v2 tasks — proving that a node stores data itself

- Status: **accepted: option A** (owner decision, 2026-11-26). Nothing new built; option D remains a possible future tier
- Date: 2026-11-26
- Written by: protocol-verifiable-participation
- Related: ADR-0003, risk K1, gap G-COMM-2, master prompt §5 / §7.3 / §18 ("no novel cryptography without formal review", consumer hardware)

## Context

v1 community tasks are Merkle (ICS-23) proofs of random state keys at recent heights. They prove a node can obtain correct chain state at the moment it answers. They do not prove the node **stores** that data. A node with no local state can fetch the same proof from any public archive or RPC node in milliseconds (K1).

The obvious next steps don't close this:
- History-serving tasks (prove an old block or tx) can be answered the same way.
- Data-availability sampling tasks can be answered the same way.
- A tight response deadline doesn't help: the chain measures time in blocks (about 1–2 s), and an archive answers in milliseconds.

To make outsourcing pointless, each node must hold a **node-specific copy** of the data that is (a) slow to produce from the public data, so it can't be made on demand, and (b) cheap for the chain to check. That is the *proof of replication* (PoRep) problem.

## Options considered

| Option | How it works | Ties data to the node? | Cost / risk |
|---|---|---|---|
| **A. Keep v1, accept K1** | No new task. Rewards pay for proven, correct, live state access, which still rejects fake nodes, copied proofs and forged data | No | None. Honest about the limit. Community rewards then measure correct participation, not storage |
| **B. Salted commitment trees** | Node commits to a Merkle root of `H(secret ‖ chunk)` when it downloads data; it is challenged on random chunks later | **No.** The node can keep only the 32-byte hashes (under 1 % of the data) and fetch chunks from an archive when challenged | Cheap, but gives a false sense of security |
| **C. "PoRep-lite": chained slow encoding** (Sloth-style modular square roots: slow to encode, fast to decode; chained so a chunk needs its predecessors) | Node stores a uniquely encoded replica; challenged on random chunks with a short block deadline; the chain decodes one chunk and checks it against the app hash | **Partly.** An attacker can store every k-th encoded chunk and recompute the rest on demand, with k ≈ deadline ÷ per-chunk encode time. At a 3 s deadline and 0.5 s per chunk the attacker stores 1/6 of the data. With 10× faster hardware (GPU), 1/60 | Novel construction with no formal security proof, and **violates the no-novel-cryptography rule** without an external review. Honest nodes spend `chunks × 0.5 s` of CPU per replica (≈ 14 h for 100 000 chunks), which is heavy for laptops |
| **D. Established PoRep** (Filecoin: stacked depth-robust graphs + SNARK proofs, audited Rust library) | Each node seals a replica; the chain verifies a SNARK per proof | **Yes**, the strongest known guarantee | Sealing needs a GPU, ≥ 128 GB RAM and hours per 32 GiB sector. **Not consumer hardware.** Large non-Go dependency and an on-chain SNARK verifier (new precompile). Months of work and an audit |
| **E. Verified peer serving** (nodes randomly challenge each other to serve data over P2P; the response is checked against the app hash on-chain) | Pays for actually delivering data to other nodes, so where the server gets it doesn't matter | n/a (it measures service, not storage) | Home nodes behind NAT can't accept inbound requests. Two identities of one operator can fake the exchange (it saves them bandwidth but doesn't raise their share of rewards). Needs a new node-to-node protocol |

## Assessment

- No option gives **cheap, consumer-hardware, outsourcing-proof storage proofs built on reviewed cryptography**. The only construction that really ties data to a node (D) needs data-center-class hardware.
- C looks like a fix but has a quantified time-memory weakness, and it would be new cryptography in consensus-adjacent code. The master prompt forbids that without formal review.
- B should be rejected outright: it looks like storage proof and isn't.

## Recommendation

1. **Now: A.** Keep the v1 tasks. State plainly in all documents that community rewards pay for *proven, correct, live state access by a bonded node*, not for storage. This is already how the officer document and the threat model describe it (K1).
2. **If storage service becomes a product goal:** add D as an **optional "archive provider" tier**, separate from consumer Community Nodes, with its own bond and reward share, using the audited Filecoin proof system (new ADR, audit, months of work).
3. Do **not** build B or C.

## Consequences
- No new attack surface or unreviewed cryptography enters the chain.
- The "community nodes store history" goal from the original idea is explicitly **not met** by consumer nodes. Storage would come from a separate professional tier (D) or from ordinary archive operators.
- Revisit if a peer-reviewed, consumer-friendly PoRep becomes available, or if the project funds the D tier.
