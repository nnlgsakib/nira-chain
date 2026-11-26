// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

// G-SEC-4: precompile state changes must roll back with the EVM call frame.
interface ICommunityLite {
    function registerNode(string calldata moniker, uint256 bond, bytes32 onionTip, uint32 onionLength)
        external returns (uint64);
}
interface IStakingLite {
    function delegate(address delegator, string memory validatorAddress, uint256 amount) external returns (bool);
}

contract Atomic {
    ICommunityLite constant COMMUNITY = ICommunityLite(0x0000000000000000000000000000000000000900);
    IStakingLite constant STAKING = IStakingLite(0x0000000000000000000000000000000000000800);
    uint256 public afterCatch;

    receive() external payable {}

    // 1. top-level revert after a successful precompile write
    function registerThenRevert(uint256 bond, bytes32 tip) external {
        COMMUNITY.registerNode("atomic", bond, tip, 100);
        revert("boom");
    }

    // 2. inner frame registers + reverts; outer frame catches and commits other state
    function inner(uint256 bond, bytes32 tip) external {
        require(msg.sender == address(this), "self only");
        COMMUNITY.registerNode("atomic", bond, tip, 100);
        STAKING.delegate(address(this), validator, bond);
        revert("inner boom");
    }

    string public validator;

    function registerInTryCatch(uint256 bond, bytes32 tip, string calldata val) external {
        validator = val;
        try this.inner(bond, tip) {
            revert("inner should have reverted");
        } catch {
            afterCatch += 1; // outer frame commits this
        }
    }
}
