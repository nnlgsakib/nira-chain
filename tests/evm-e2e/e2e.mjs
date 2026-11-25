// EVM end-to-end checks for a running Nira node (R-F9, R-F10, R-F11).
// Usage: RPC=http://127.0.0.1:8545 KEY=<hex> COSMOS_REST=http://127.0.0.1:1317 node e2e.mjs
import { readFileSync } from "node:fs";
import solc from "solc";
import { ethers } from "ethers";
import assert from "node:assert/strict";

const RPC = process.env.RPC ?? "http://127.0.0.1:8545";
const REST = process.env.COSMOS_REST ?? "http://127.0.0.1:1317";
const EXPECTED_CHAIN_ID = BigInt(process.env.CHAIN_ID ?? 47201);
const provider = new ethers.JsonRpcProvider(RPC);
const wallet = new ethers.Wallet(process.env.KEY, provider);
let passed = 0;
const check = async (name, fn) => { await fn(); passed++; console.log(`ok  ${name}`); };

const src = readFileSync(new URL("./Counter.sol", import.meta.url), "utf8");
const out = JSON.parse(solc.compile(JSON.stringify({
  language: "Solidity", sources: { "Counter.sol": { content: src } },
  settings: { evmVersion: "cancun", outputSelection: { "*": { "*": ["abi", "evm.bytecode.object"] } } },
})));
const C = out.contracts["Counter.sol"].Counter;

await check("eth_chainId matches protocol constant", async () => {
  assert.equal((await provider.getNetwork()).chainId, EXPECTED_CHAIN_ID);
});

await check("blocks advance", async () => {
  const a = await provider.getBlockNumber();
  await new Promise(r => setTimeout(r, 2500));
  assert.ok((await provider.getBlockNumber()) > a);
});

await check("EVM balance == bank balance (single state)", async () => {
  const evmBal = await provider.getBalance(wallet.address);
  const bech = await (await fetch(`${REST}/cosmos/evm/vm/v1/cosmos_account/${wallet.address}`)).json();
  const bank = await (await fetch(`${REST}/cosmos/bank/v1beta1/balances/${bech.cosmos_address}/by_denom?denom=anira`)).json();
  assert.equal(evmBal, BigInt(bank.balance.amount));
});

let counter;
await check("deploy Solidity contract + receipt", async () => {
  const f = new ethers.ContractFactory(C.abi, C.evm.bytecode.object, wallet);
  counter = await f.deploy();
  const rcpt = await counter.deploymentTransaction().wait();
  assert.equal(rcpt.status, 1);
  assert.ok((await provider.getCode(await counter.getAddress())).length > 2);
});

await check("EIP-1559 tx, logs, event filter", async () => {
  const tx = await counter.inc(5, { type: 2 });
  const rcpt = await tx.wait();
  assert.equal(rcpt.status, 1);
  assert.equal(rcpt.logs.length, 1);
  const ev = counter.interface.parseLog(rcpt.logs[0]);
  assert.equal(ev.args.newCount, 5n);
  assert.equal(await counter.count(), 5n);
  const logs = await counter.queryFilter(counter.filters.Incremented(wallet.address), rcpt.blockNumber, rcpt.blockNumber);
  assert.equal(logs.length, 1);
  const byHash = await provider.getTransaction(tx.hash);
  assert.equal(byHash.hash, tx.hash);
});

await check("revert with custom error is reported, state unchanged", async () => {
  await assert.rejects(counter.inc.staticCall(1000), e => e.revert?.name === "TooLarge");
  const gas = await counter.inc.estimateGas(1);
  assert.ok(gas > 21000n && gas < 200000n);
  assert.equal(await counter.count(), 5n);
});

await check("legacy (type 0) native transfer, nonce increments", async () => {
  const n0 = await provider.getTransactionCount(wallet.address);
  const to = ethers.Wallet.createRandom().address;
  const tx = await wallet.sendTransaction({ to, value: 12345n, type: 0, gasPrice: (await provider.getFeeData()).gasPrice });
  await tx.wait();
  assert.equal(await provider.getBalance(to), 12345n);
  assert.equal(await provider.getTransactionCount(wallet.address), n0 + 1);
});

await check("fee data (EIP-1559 base fee present)", async () => {
  const b = await provider.getBlock("latest");
  assert.ok(b.baseFeePerGas !== null);
});

await check("staking precompile readable from EVM", async () => {
  const vals = await (await fetch(`${REST}/cosmos/staking/v1beta1/validators`)).json();
  const valoper = vals.validators[0].operator_address;
  const st = new ethers.Contract("0x0000000000000000000000000000000000000800",
    ["function delegation(address,string) view returns (uint256 shares, (string denom, uint256 amount) balance)"], provider);
  const [, bal] = await st.delegation(wallet.address, valoper);
  assert.equal(bal.denom, "anira");
});

console.log(`\n${passed} checks passed`);
