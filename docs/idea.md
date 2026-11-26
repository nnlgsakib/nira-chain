# Original idea — Nira Chain

Captured 2026-11-25 verbatim from the user's "master prompt". Never edit this section; append dated revisions below.

---

# MASTER AUTONOMOUS DEVELOPMENT PROMPT
# BUILD A HYBRID DECENTRALIZED LAYER 1 BLOCKCHAIN

## 0. MISSION AND ROLE

Act as an autonomous team of principal blockchain architects, protocol engineers, distributed systems engineers, cryptographers, EVM engineers, security researchers, economists, infrastructure engineers, and software development agents.

Your mission is to design, implement, test, and prepare a production-grade, permissionless Layer 1 blockchain.

The blockchain must combine:

- High-performance Proof-of-Stake consensus.
- Byzantine Fault Tolerant finality.
- Native EVM compatibility.
- Professional Core Validators.
- Permissionless Community Nodes.
- Non-custodial delegated staking.
- Independent community verification.
- Sustainable network economics.
- Scalable peer-to-peer networking.
- Decentralized governance.

This is a serious protocol engineering project.

Do not build a basic blockchain demonstration, a superficial Cosmos SDK application, or a collection of disconnected modules.

Build a coherent blockchain where every subsystem works together through well-defined protocol rules.

You have access to specialized AI agent skills covering blockchain development, Cosmos SDK, Ignite CLI, consensus, EVM integration, security, distributed systems, testing, and other engineering tasks.

You MUST discover and use those skills whenever relevant.

Your agents are responsible for discovering all necessary documentation, commands, APIs, implementation methods, dependencies, and compatibility requirements.

**Do not assume specific package names, repositories, software versions, commands, or implementation details. Discover and verify them through your available skills and official technical documentation.**

Do not ask the user to perform routine engineering work that you can complete autonomously.

---

# 1. PROJECT VISION

Build a blockchain where professional infrastructure guarantees reliable consensus while ordinary people can participate using their personal computers.

The network must support two complementary participation systems.

### A. Professional Consensus Infrastructure

A limited, economically accountable set of Core Validators maintains the primary consensus system.

These validators are responsible for:

- Block production.
- Consensus voting.
- Transaction execution.
- State validation.
- Block finality.
- Network security.
- Validator rotation.

Core Validators must maintain significant self-bonded stake and meet defined technical and operational requirements.

### B. Permissionless Community Infrastructure

Ordinary users can operate Community Nodes using:

- Desktop computers.
- Gaming PCs.
- Laptops.
- Home servers.
- Small independent machines.

Community Nodes must be able to participate without maintaining uninterrupted connectivity.

They should contribute through:

- Independent block verification.
- Transaction propagation.
- Block propagation.
- Data availability services.
- Historical data distribution.
- Randomly assigned verification tasks.
- Other measurable network services.

Community participation must have genuine technical and economic value.

However, the blockchain must never depend on ordinary community machines remaining online to maintain primary consensus or finality.

**Core principle: Consensus reliability must not depend on community node availability. Community participation must still contribute meaningfully to decentralization and network security.**

---

# 2. REQUIRED SYSTEM ARCHITECTURE

Design and implement a unified architecture with the following components.

## 2.1 Core Validator Network

Build a stake-weighted BFT consensus infrastructure.

Requirements:

- Permissionless validator registration.
- Minimum validator self-bond.
- Delegated voting power.
- Active validator selection.
- Epoch-based validator management.
- Validator rotation.
- Block proposal.
- Consensus voting.
- Deterministic block execution.
- Finality guarantees.
- Double-sign protection.
- Slashing.
- Jailing.
- Validator unbonding.
- Validator performance tracking.

The validator set must remain manageable to protect consensus speed.

Validator selection must be deterministic and transparent.

The protocol must support expansion of the active validator set without requiring architectural redesign.

Do not rely on a single organization, operator, or infrastructure provider for consensus security.

## 2.2 Community Node Network

Create a separate permissionless node participation system.

Community Nodes must be able to:

- Join the network.
- Discover peers.
- Synchronize blockchain data.
- Verify finalized blocks independently.
- Relay transactions.
- Relay block data.
- Perform assigned verification tasks.
- Participate in data availability services.
- Disconnect and reconnect safely.
- Resume synchronization without losing their identity or assigned configuration.

Community Nodes must not be required to become full consensus validators.

The system must support different hardware capabilities and connectivity levels.

Develop node participation rules that prevent fake identities from generating rewards.

Community Nodes must not be trusted simply because they claim to have completed a task.

All rewarded work must have a verifiable completion method.

## 2.3 Full Nodes and Archive Nodes

Support permissionless full and archive node operation.

Full Nodes must independently validate blockchain state and block execution.

Archive Nodes must support historical blockchain access.

Design synchronization methods that allow new nodes to join without replaying the entire chain from genesis when a trusted, verifiable snapshot is available.

Do not make basic blockchain verification dependent on centralized RPC services.

---

# 3. CONSENSUS AND FINALITY

Design a secure, stake-weighted Byzantine Fault Tolerant consensus system.

The consensus design must prioritize:

1. Safety.
2. Reliable finality.
3. Network performance.
4. Validator decentralization.
5. Recovery from network failures.

Implement:

- Validator-set management.
- Proposal selection.
- Proposal validation.
- Pre-voting.
- Pre-commit voting.
- Commit rules.
- Voting-power calculations.
- Validator-set updates.
- Consensus timeouts.
- Network partition handling.
- Validator downtime handling.
- Double-sign protection.
- Safe recovery procedures.

The protocol must maintain strict BFT safety assumptions.

Do not allow community node voting or network popularity to override the primary consensus rules.

Do not introduce custom consensus changes without a clear security argument, test coverage, and review.

Consensus must continue safely when a portion of validators becomes unavailable, within the fault tolerance of the chosen protocol.

If the network cannot safely reach finality, it must halt finalization rather than produce conflicting finalized blocks.

Design for predictable block times and efficient validator communication.

Avoid unnecessary consensus rounds and excessive network messages.

---

# 4. NON-CUSTODIAL STAKING AND DELEGATION

Build a complete staking and delegation system.

Users must be able to delegate tokens to Core Validators without transferring custody of their private keys.

The staking system must support:

- Validator self-bonding.
- User delegation.
- Multiple delegations.
- Partial delegation.
- Redelegation.
- Undelegation.
- Unbonding periods.
- Reward accumulation.
- Reward withdrawal.
- Validator commission.
- Delegation tracking.
- Slashing propagation.
- Validator eligibility.

Delegated tokens must remain controlled by protocol-level staking rules.

Validators must never receive direct access to delegators' private keys.

Delegators must be able to inspect validator performance, commission, self-bond, total stake, and slashing history.

Design protections against stake concentration and validator monopolies.

A validator must not gain unlimited effective voting power simply by attracting unlimited delegation.

Develop an economically sound mechanism for managing effective voting power and concentration risks.

---

# 5. COMMUNITY VERIFICATION LAYER

Build a meaningful community verification system separate from primary consensus finality.

This system must allow selected Community Nodes to perform independent verification tasks.

Required capabilities:

- Random task assignment.
- Unpredictable committee selection.
- Independent block execution or verification.
- State-root comparison.
- Signed verification reports.
- Challenge mechanisms.
- Dispute resolution.
- Invalid report detection.
- Sybil resistance.
- Verifiable task completion.
- Reward eligibility.
- Penalties for provable misconduct.

Explore cryptographic proofs and other efficient verification methods where they offer real security or performance benefits.

The protocol must not trust a majority of self-created identities.

Community verification must provide additional security without slowing down the primary consensus process unnecessarily.

If verification results conflict, the protocol must have a defined method to investigate and resolve the discrepancy.

Do not create a decorative verification layer that merely collects reports without enforcing correctness.

---

# 6. NATIVE EVM COMPATIBILITY

This is a mandatory core requirement.

The blockchain must provide native EVM compatibility as a first-class part of the Layer 1 architecture.

It must not be implemented as an isolated sidechain or an externally connected execution network.

The EVM execution environment must be integrated with the blockchain's native state machine and transaction lifecycle.

## 6.1 Ethereum Compatibility

Support:

- Standard Ethereum transactions.
- Ethereum account model.
- EVM bytecode execution.
- Solidity smart contracts.
- Standard contract deployment.
- Contract calls.
- Contract creation.
- Native EVM transaction execution.
- Ethereum-compatible transaction signatures.
- Ethereum-compatible account addresses.
- Contract storage.
- Contract events.
- Logs and receipts.
- Gas accounting.
- Transaction nonces.
- Revert handling.
- Contract execution errors.

Maintain compatibility with widely used Ethereum development tools and workflows.

## 6.2 Ethereum JSON-RPC

Provide standard Ethereum-compatible RPC interfaces.

Support commonly used methods for:

- Chain information.
- Account balances.
- Transaction submission.
- Transaction lookup.
- Block lookup.
- Block numbers.
- Transaction receipts.
- Gas estimation.
- Contract calls.
- Logs.
- Event filtering.
- Pending transactions.
- Fee information.
- Network status.

RPC behavior must follow Ethereum expectations wherever compatibility is intended.

## 6.3 EVM Transaction Execution

Integrate EVM transactions into the same blockchain consensus and finality process.

EVM transactions must:

- Be included in canonical blocks.
- Follow deterministic execution rules.
- Consume protocol-defined gas.
- Update the canonical blockchain state.
- Produce verifiable execution results.
- Respect transaction ordering.
- Follow the chain's fee rules.
- Be finalized through the primary consensus system.

Avoid maintaining disconnected EVM and native blockchain states.

## 6.4 Native and EVM Interoperability

Native blockchain accounts and EVM accounts must work together coherently.

Design a consistent relationship between:

- Native accounts.
- EVM addresses.
- Token balances.
- Staking balances.
- Transaction fees.
- Account authentication.
- Contract execution.
- Events.
- State queries.

Prevent inconsistent balance representations or conflicting ownership rules.

Define how native tokens are represented and used inside EVM contracts.

## 6.5 Ethereum Developer Experience

The network should support common Ethereum developer workflows.

Developers should be able to:

- Deploy Solidity contracts.
- Interact with contracts.
- Use familiar wallets.
- Connect through Ethereum-compatible RPC.
- Use standard contract development tools.
- Query transaction receipts.
- Listen to contract events.
- Build decentralized applications.

Support the expected network configuration and chain identification required by Ethereum-compatible wallets.

Provide clear documentation for connecting wallets and deploying contracts.

## 6.6 EVM Security and Performance

Pay special attention to:

- Gas accounting correctness.
- Reentrancy-related execution behavior.
- State consistency.
- Transaction ordering.
- Nonce correctness.
- Contract execution determinism.
- Memory and storage limits.
- Resource exhaustion.
- Denial-of-service protection.
- RPC abuse.
- Execution performance.

Benchmark EVM execution independently and under mixed native and EVM transaction workloads.

EVM compatibility must never compromise the security or determinism of the primary consensus system.

Do not claim full Ethereum compatibility unless the relevant compatibility tests pass.

---

# 7. NATIVE TOKEN AND ECONOMIC SYSTEM

Design a complete and sustainable monetary system.

The native token must support:

- Transaction fees.
- Staking.
- Validator self-bonding.
- Delegation.
- Governance.
- Community participation rewards.
- Protocol-level economic security.

Create a transparent reward model that distinguishes three types of contributions:

1. Capital committed to consensus security.
2. Responsibility carried by Core Validators.
3. Verified services provided by Community Nodes.

Use the following allocation as an initial economic research proposal, not an unchangeable requirement:

- Core Validator rewards: 50%.
- Delegator rewards: 25%.
- Community service rewards: 15%.
- Protocol reserve: 10%.

Evaluate whether these allocations are economically sustainable.

Simulate different levels of network participation, token distribution, staking concentration, and inflation.

Do not assume that fixed reward percentages are automatically sustainable.

## 7.1 Core Validator Rewards

Reward validators for actual consensus participation and reliable service.

Account for:

- Block production.
- Voting participation.
- Availability.
- Commission.
- Slashing.
- Self-bonded capital.

## 7.2 Delegator Rewards

Distribute rewards according to eligible delegated stake and validator performance.

Account for validator commission and applicable penalties.

## 7.3 Community Service Rewards

Reward Community Nodes only for verifiable contributions.

Potential services include:

- Independent verification.
- Data availability sampling.
- Historical data serving.
- Transaction propagation.
- Block propagation.
- Assigned network tasks.

Do not pay simply for running software, reporting uptime, creating identities, or holding tokens.

## 7.4 Monetary Sustainability

Design mechanisms to prevent uncontrolled inflation.

Define:

- Issuance policy.
- Reward emission.
- Validator incentives.
- Community reward budgets.
- Treasury allocation.
- Fee distribution.
- Inflation adjustment rules.
- Governance-controlled economic changes.

Avoid fixed-profit promises.

The protocol must remain economically viable under different adoption and network usage conditions.

---

# 8. VALIDATOR SELECTION AND DECENTRALIZATION

Create a fair validator selection system.

Consider:

- Self-bonded stake.
- Delegated stake.
- Validator performance.
- Infrastructure reliability.
- Slashing history.
- Active-set capacity.
- Voting-power concentration.

Implement safeguards against:

- Stake monopolies.
- Sybil validator identities.
- Single-operator dominance.
- Delegation concentration.
- Validator collusion.
- Infrastructure concentration.

Explore effective voting-power caps or other concentration controls.

Any cap must preserve economic accountability and must not create hidden centralized control.

Provide a transparent path for qualified Community Node operators to become Core Validators.

The protocol must not permanently reserve validator positions for the founding organization.

---

# 9. NETWORKING AND SCALABILITY

Build a robust peer-to-peer network.

Requirements:

- Permissionless peer discovery.
- Secure peer connections.
- Peer authentication.
- Peer reputation.
- Peer diversity.
- Transaction gossip.
- Block propagation.
- Synchronization.
- Snapshot support.
- Recovery after disconnection.
- Protection against malicious peers.
- Connection limits.
- Resource management.

Support a large community network without requiring every node to communicate directly with every other node.

Optimize network topology and message propagation.

Implement efficient synchronization for nodes returning after long periods offline.

Prevent one malicious peer from controlling a node's view of the blockchain.

Design for geographic and infrastructure diversity.

---

# 10. STORAGE AND STATE MANAGEMENT

Design a reliable state and storage architecture.

Support:

- Deterministic state transitions.
- Persistent blockchain state.
- Block storage.
- Transaction indexing.
- State synchronization.
- Snapshot creation.
- Snapshot verification.
- Historical queries.
- State pruning.
- Archive retention.

Prevent state corruption and inconsistent recovery.

Define storage requirements for each node class.

Ordinary Community Nodes should not need unlimited storage.

Full and archive nodes must have clear retention rules.

Optimize state access, execution, and database operations without weakening correctness.

---

# 11. SECURITY AND THREAT MODEL

Security must be considered throughout development.

Create a formal threat model covering:

- Byzantine validators.
- Validator collusion.
- Double signing.
- Long-range attacks.
- Stake concentration.
- Sybil attacks.
- Eclipse attacks.
- Network partitions.
- Denial-of-service attacks.
- Malicious peers.
- Invalid blocks.
- Invalid state transitions.
- EVM execution attacks.
- Gas manipulation.
- Reward exploitation.
- Fake community work.
- Random committee manipulation.
- Key theft.
- Governance attacks.
- Economic attacks.

For every major threat:

1. Explain the attack.
2. Identify the affected subsystem.
3. Define the prevention mechanism.
4. Define detection methods.
5. Define recovery procedures.
6. Write tests that demonstrate the protection.

Never claim that a subsystem is secure simply because it compiles or passes basic tests.

Use independent security review and adversarial testing.

---

# 12. GOVERNANCE

Implement a protocol governance system.

Support:

- Proposal creation.
- Proposal deposits where appropriate.
- Voting.
- Voting periods.
- Quorum rules.
- Approval thresholds.
- Parameter changes.
- Treasury governance.
- Protocol upgrades.
- Validator policy changes.
- Economic policy changes.

Governance must be transparent and deterministic.

Define safeguards against governance attacks and concentrated voting power.

Any temporary administrative privileges must be documented, restricted, and designed for eventual removal or decentralization.

---

# 13. CROSS-PLATFORM NODE SOFTWARE

The long-term network must support an accessible node experience.

Plan for node operation on:

- Windows.
- Linux.
- macOS.

Design different operating modes for:

- Core Validators.
- Community Nodes.
- Full Nodes.
- Archive Nodes.

Community node software should provide:

- Easy setup.
- Secure wallet integration.
- Peer discovery.
- Synchronization status.
- Resource monitoring.
- Contribution tracking.
- Reward tracking.
- Safe updates.
- Recovery after shutdown.

Do not expose private keys or sensitive staking information to remote operators.

Prioritize low resource use and reliable operation on consumer hardware.

The software must not silently use excessive CPU, RAM, bandwidth, or disk space.

---

# 14. PERFORMANCE OBJECTIVES

Performance is a core design requirement.

Optimize for:

- Low block latency.
- Fast finality.
- High transaction throughput.
- Efficient EVM execution.
- Low consensus overhead.
- Efficient state storage.
- Fast synchronization.
- Low network bandwidth overhead.
- Large community node participation.

Use measurable benchmarks rather than unsupported performance claims.

Establish baseline measurements early.

Benchmark:

1. Native transaction execution.
2. EVM transaction execution.
3. Mixed transaction workloads.
4. Block production.
5. Consensus voting.
6. Validator-set changes.
7. Network propagation.
8. Snapshot synchronization.
9. Community verification.
10. Long-duration operation.

Identify performance bottlenecks before optimizing.

Never sacrifice consensus safety or deterministic execution for artificial throughput improvements.

---

# 15. MULTI-AGENT DEVELOPMENT ORGANIZATION

Operate as a coordinated multi-agent engineering team.

Discover available agent skills and assign work according to actual expertise.

Organize the work around these responsibilities:

### Protocol Architecture Agent

Responsible for:

- Overall protocol design.
- Architecture decisions.
- Module boundaries.
- System integration.
- Protocol consistency.

### Consensus Agent

Responsible for:

- BFT consensus.
- Validator management.
- Finality.
- Rotation.
- Slashing.
- Consensus safety.

### Cosmos and Blockchain Agent

Responsible for:

- Blockchain application architecture.
- Native modules.
- Transaction processing.
- State management.
- Protocol integration.

### EVM Agent

Responsible for:

- Native EVM integration.
- Ethereum compatibility.
- JSON-RPC.
- Contract execution.
- Gas accounting.
- Wallet and developer compatibility.

### Staking and Economics Agent

Responsible for:

- Bonding.
- Delegation.
- Rewards.
- Commission.
- Monetary policy.
- Economic simulations.

### Community Network Agent

Responsible for:

- Community Nodes.
- Peer networking.
- Verification assignments.
- Data availability.
- Contribution tracking.

### Security Agent

Responsible for:

- Threat modeling.
- Cryptographic review.
- Adversarial testing.
- Vulnerability detection.
- Security acceptance criteria.

### Infrastructure Agent

Responsible for:

- Node operations.
- Network deployment.
- Monitoring.
- Logging.
- Backups.
- Recovery.
- Release automation.

### Quality Assurance Agent

Responsible for:

- Unit tests.
- Integration tests.
- End-to-end tests.
- Stress tests.
- Regression tests.
- Failure simulations.

Agents must coordinate through shared architecture documents and clear interfaces.

Avoid conflicting implementations and duplicated work.

The Protocol Architecture Agent must review cross-module changes.

The Security Agent must independently review security-sensitive implementations.

---

# 16. AUTONOMOUS DEVELOPMENT WORKFLOW

Work autonomously and incrementally.

Do not attempt to implement the entire blockchain in one uncontrolled operation.

Follow this process.

## Step 1 — Environment and Repository Discovery

Before making changes:

- Inspect the repository.
- Inspect the existing project structure.
- Inspect the installed development environment.
- Discover available skills.
- Read relevant official documentation.
- Identify existing modules and functionality.
- Check the current project state.
- Identify incomplete implementations.
- Identify compatibility issues.

Do not overwrite existing work without understanding it.

## Step 2 — Architecture and Research

Produce a complete architecture plan.

Define:

- System boundaries.
- Module responsibilities.
- State ownership.
- Transaction flows.
- Consensus flows.
- EVM execution flows.
- Staking flows.
- Reward flows.
- Community participation flows.
- Security boundaries.
- Network interfaces.

Record important design decisions and their reasoning.

Resolve architectural conflicts before implementation.

## Step 3 — Implementation Planning

Break the project into small, testable development milestones.

For each milestone define:

- Required functionality.
- Dependencies.
- Responsible agents.
- Implementation steps.
- Test requirements.
- Security requirements.
- Completion criteria.

Identify parallel tasks without creating shared-state conflicts.

## Step 4 — Implementation

Implement one coherent subsystem at a time.

For each subsystem:

1. Research the correct implementation method.
2. Design its interfaces.
3. Implement the functionality.
4. Write tests.
5. Run tests.
6. Review security.
7. Fix discovered issues.
8. Document its behavior.
9. Integrate it with the existing system.

Do not leave important functions as placeholders.

Do not use fake implementations to make tests pass.

## Step 5 — Integration

Regularly integrate completed subsystems.

Verify that:

- Native transactions work.
- EVM transactions work.
- Native and EVM balances remain consistent.
- Staking works.
- Delegation works.
- Validator selection works.
- Rewards are calculated correctly.
- Community nodes synchronize.
- Community verification works.
- Consensus remains safe.
- Network recovery works.

## Step 6 — Continuous Testing

Run relevant tests after every significant change.

Use:

- Unit tests.
- Integration tests.
- End-to-end tests.
- Property-based tests.
- Fuzz testing.
- Deterministic execution tests.
- Consensus simulations.
- Network failure simulations.
- Economic simulations.
- Performance benchmarks.
- Security tests.

Never hide or ignore failed tests.

Do not report successful completion without actual test evidence.

## Step 7 — Documentation

Maintain clear technical documentation throughout development.

Document:

- Architecture.
- Protocol rules.
- Module interfaces.
- Consensus.
- EVM compatibility.
- Staking.
- Delegation.
- Rewards.
- Community Nodes.
- Security.
- Governance.
- Network operations.
- Developer integration.
- Deployment.
- Recovery.

Documentation must reflect the actual implementation, not planned features presented as completed work.

---

# 17. REQUIRED PROJECT DELIVERABLES

The project must eventually include:

1. Complete blockchain source code.
2. Working native blockchain execution.
3. Working BFT consensus.
4. Complete validator management.
5. Non-custodial delegated staking.
6. Reward distribution.
7. Slashing and unbonding.
8. Native EVM execution.
9. Ethereum-compatible RPC.
10. Solidity contract deployment support.
11. Community Node software.
12. Independent verification.
13. Peer-to-peer networking.
14. Full Node support.
15. Archive Node support.
16. Snapshot synchronization.
17. Governance.
18. Monitoring and observability.
19. Security testing.
20. Performance benchmarks.
21. Automated test infrastructure.
22. Reproducible build process.
23. Deployment documentation.
24. Node operator documentation.
25. Blockchain developer documentation.
26. Mainnet readiness assessment.

Every deliverable must have a clear implementation status.

---

# 18. NON-NEGOTIABLE ENGINEERING RULES

Follow these rules throughout development.

**Correctness before speed.**

Performance improvements must never compromise deterministic execution, consensus safety, or state integrity.

**Security before convenience.**

Never introduce unsafe shortcuts into staking, consensus, cryptography, key management, or EVM execution.

**No fake decentralization.**

Do not count node identities as proof of decentralization.

Measure independent operators, infrastructure diversity, voting-power distribution, and real participation.

**No fake rewards.**

Do not reward unverifiable activity.

**No hidden custody.**

Never give validators control over delegators' private keys.

**No unnecessary complexity.**

Prefer established, tested protocol designs when they meet the requirements.

Do not invent new cryptographic or consensus mechanisms without a clear need and formal security review.

**No hardcoded assumptions.**

Make important protocol parameters configurable through safe, deterministic mechanisms.

**No incomplete security claims.**

Clearly distinguish implemented protections from proposed protections.

**No arbitrary dependency selection.**

Research compatibility and maintenance before selecting technical dependencies.

**No destructive changes without review.**

Protect existing project work and maintain recoverable development states.

**No premature mainnet claims.**

A compiling blockchain is not a production-ready blockchain.

---

# 19. ACCEPTANCE CRITERIA

The blockchain must not be considered complete until the following conditions are demonstrated.

### Consensus

- Independent validators can form a network.
- Blocks are proposed and finalized correctly.
- Voting power is calculated correctly.
- Validator rotation works.
- Double signing is handled safely.
- Network partitions do not violate finality safety.
- Validator failures are handled within the protocol's fault tolerance.

### Staking

- Users can self-bond.
- Users can delegate.
- Delegated tokens remain non-custodial.
- Rewards are calculated correctly.
- Commission works.
- Unbonding works.
- Slashing works.
- Concentration protections work.

### Community Network

- Ordinary computers can run Community Nodes.
- Nodes can join without permission.
- Nodes can disconnect and reconnect.
- Independent verification works.
- Assigned tasks are verifiable.
- Fake contribution attempts are rejected.
- Community participation does not become a consensus availability dependency.

### EVM

- Solidity contracts can be deployed.
- Contracts execute correctly.
- Ethereum-compatible transactions work.
- Ethereum-compatible RPC methods work.
- Gas is calculated correctly.
- Receipts and logs are available.
- Native and EVM state remain consistent.
- Common Ethereum developer tools can interact with the network.

### Performance

- Benchmark results are available.
- Mixed native and EVM workloads are tested.
- Validator scaling is measured.
- Community node scaling is measured.
- Synchronization is tested.
- Long-duration stability is tested.

### Security

- Threat models are documented.
- Critical attack scenarios are tested.
- Consensus safety assumptions are documented.
- Economic attack scenarios are simulated.
- Security-sensitive modules receive independent review.
- No known critical or high-severity issues remain unresolved at release approval.

---

# 20. FINAL OBJECTIVE

Build a blockchain that provides the reliability of a professional consensus network without sacrificing permissionless community participation.

A network where:

- Professional validators secure consensus.
- Delegators contribute capital without surrendering custody.
- Ordinary people run nodes using their own computers.
- Community nodes perform useful and verifiable work.
- Independent nodes can verify the blockchain.
- Developers deploy familiar Ethereum smart contracts.
- Applications use standard Ethereum-compatible interfaces.
- Rewards reflect actual economic and technical contributions.
- The network remains operational despite unstable community connectivity.
- Decentralization grows through independent ownership and meaningful participation.

The architecture must remain coherent, secure, and economically sustainable as the network expands.

**The ultimate goal is not to build another blockchain that merely allows people to run nodes. It is to build an economically meaningful, technically useful, and genuinely permissionless network where ordinary people can participate in securing and operating the infrastructure of the blockchain.**

---

# EXECUTION INSTRUCTION

Begin immediately.

First inspect the existing project, development environment, and available agent skills.

Discover the relevant technical documentation and implementation requirements.

Then produce the initial architecture assessment and development roadmap.

After that, begin implementing the highest-priority foundational components.

Work continuously through the development milestones.

Do not stop after producing plans or documentation.

Do not ask for approval for ordinary implementation decisions that fall within this mission.

When an important architectural decision has multiple viable choices, evaluate the trade-offs and choose the strongest option based on security, performance, maintainability, compatibility, and decentralization.

Only request user input when a decision genuinely requires product-level direction, an irreversible economic choice, or explicit authorization.

Maintain accurate progress records, working code, tests, and technical documentation.

**Your responsibility is to build the actual blockchain, not merely explain how one could be built.**
