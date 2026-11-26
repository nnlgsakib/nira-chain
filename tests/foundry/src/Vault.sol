// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

/// Minimal native-token vault used by the Foundry e2e (tests/foundry/run.sh):
/// deposits NIRA (msg.value), emits events, supports withdraw, and can delegate
/// its own balance through the staking precompile.
interface IStaking {
    function delegate(address delegator, string memory validatorAddress, uint256 amount) external returns (bool);
}

contract Vault {
    event Deposited(address indexed from, uint256 amount);
    event Withdrawn(address indexed to, uint256 amount);
    mapping(address => uint256) public balanceOf;

    receive() external payable { deposit(); }

    function deposit() public payable {
        balanceOf[msg.sender] += msg.value;
        emit Deposited(msg.sender, msg.value);
    }

    function withdraw(uint256 amount) external {
        require(balanceOf[msg.sender] >= amount, "insufficient");
        balanceOf[msg.sender] -= amount;
        (bool ok,) = msg.sender.call{value: amount}("");
        require(ok, "send failed");
        emit Withdrawn(msg.sender, amount);
    }

    function stake(string calldata validator, uint256 amount) external {
        require(IStaking(0x0000000000000000000000000000000000000800).delegate(address(this), validator, amount), "delegate");
    }
}
