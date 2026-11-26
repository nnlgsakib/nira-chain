// Prints the delegation amount of <delegator hex> to <valoper> read through the
// EVM staking precompile (0x...0800). Used by tests/cosmos-e2e for
// cross-interface checks. Usage: node precompile-delegation.mjs <0xaddr> <valoper>
import { ethers } from "ethers";

const [, , delegator, valoper] = process.argv;
const provider = new ethers.JsonRpcProvider(process.env.EVM_RPC ?? "http://127.0.0.1:8545");
const st = new ethers.Contract("0x0000000000000000000000000000000000000800",
  ["function delegation(address,string) view returns (uint256 shares, (string denom, uint256 amount) balance)"],
  provider);
const [, bal] = await st.delegation(delegator, valoper);
process.stdout.write(bal.amount.toString());
