// Hardhat 3 against a running Nira localnet (G-TEST-3). KEY = dev1's eth key.
import hardhatToolboxMochaEthers from "@nomicfoundation/hardhat-toolbox-mocha-ethers";

export default {
  plugins: [hardhatToolboxMochaEthers],
  solidity: { version: "0.8.28", settings: { evmVersion: "cancun" } },
  networks: {
    nira: {
      type: "http",
      chainType: "l1",
      url: process.env.EVM_RPC ?? "http://127.0.0.1:8545",
      chainId: 93000,
      accounts: process.env.KEY ? [process.env.KEY.startsWith("0x") ? process.env.KEY : "0x" + process.env.KEY] : [],
    },
  },
};
