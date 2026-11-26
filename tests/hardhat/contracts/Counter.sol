// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

contract Counter {
    uint256 public count;
    event Incremented(address indexed by, uint256 newCount);
    error TooLarge(uint256 by);

    function inc(uint256 by) external {
        if (by > 100) revert TooLarge(by);
        count += by;
        emit Incremented(msg.sender, count);
    }
}

interface IStaking {
    function delegation(address delegator, string memory validator)
        external view returns (uint256 shares, Coin memory balance);
}
struct Coin { string denom; uint256 amount; }
