// SPDX-License-Identifier: MIT
pragma solidity >=0.8.18;

/// @dev Nira x/community precompile (ADR-0006, docs/spec/community.md §10).
address constant COMMUNITY_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000900;

ICommunity constant COMMUNITY_CONTRACT = ICommunity(COMMUNITY_PRECOMPILE_ADDRESS);

struct NodeInfo {
    string moniker;
    uint256 bond;
    /// 1 = ACTIVE, 2 = UNBONDING
    uint8 status;
    uint64 registeredEpoch;
    uint64 onionBaseEpoch;
    uint64 unitsTotal;
    uint32 unitsThisEpoch;
    int64 unbondCompleteHeight;
}

struct EpochInfo {
    uint64 number;
    int64 startHeight;
    int64 nextEpochHeight;
    bytes32 seed;
    uint256 eligibleBond;
    uint256 budget;
    uint256 totalActiveBond;
}

struct TaskInfo {
    uint32 index;
    string store;
    int64 height;
    bytes32 key;
}

/// @title ICommunity — community node participation from Solidity / Ethereum wallets.
/// The operator is always msg.sender (an EOA or a contract, e.g. a node pool).
/// Every call runs the same keeper code as the Cosmos messages.
interface ICommunity {
    event NodeRegistered(address indexed operator, uint256 bond, uint64 baseEpoch);
    event OnionRotated(address indexed operator, uint64 baseEpoch);
    event TaskProven(address indexed operator, uint64 indexed epoch, uint32 taskIndex);
    event NodeDeregistered(address indexed operator, int64 unbondCompleteHeight);
    event DivergenceReported(address indexed operator, uint64 indexed alertId, int64 height);

    /// @notice Bond `bond` (base units, 18 decimals) of the native token and register.
    function registerNode(string calldata moniker, uint256 bond, bytes32 onionTip, uint32 onionLength)
        external returns (uint64 baseEpoch);

    function rotateOnion(bytes32 onionTip, uint32 onionLength) external returns (uint64 baseEpoch);

    /// @param proof protobuf-encoded ibc.core.commitment.v1.MerkleProof
    function submitTaskProof(
        uint64 epoch,
        bytes32 reveal,
        uint32 taskIndex,
        string calldata store,
        int64 height,
        bytes calldata value,
        bytes calldata proof
    ) external returns (bool success);

    function deregisterNode() external returns (int64 unbondCompleteHeight);

    function reportDivergence(int64 height, bytes32 claimedAppHash) external returns (uint64 alertId);

    function getNode(address operator) external view returns (NodeInfo memory node);

    function currentEpoch() external view returns (EpochInfo memory epoch);

    function getTasks(address operator) external view returns (TaskInfo[] memory tasks);
}
