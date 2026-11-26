// EVM end-to-end checks for a running Nira node (R-F9, R-F10, R-F11).
// Usage: RPC=http://127.0.0.1:8545 KEY=<hex> COSMOS_REST=http://127.0.0.1:1317 node e2e.mjs
import { readFileSync } from "node:fs";
import solc from "solc";
import { ethers } from "ethers";
import assert from "node:assert/strict";

const RPC = process.env.RPC ?? "http://127.0.0.1:8545";
const REST = process.env.COSMOS_REST ?? "http://127.0.0.1:1317";
const EXPECTED_CHAIN_ID = BigInt(process.env.CHAIN_ID ?? 47201);
const provider = new ethers.JsonRpcProvider(RPC, undefined, { cacheTimeout: -1 }); // no response caching: nonce/balance reads must be fresh
const wallet = new ethers.Wallet(process.env.KEY, provider);
let passed = 0;
const CHECK_TIMEOUT_MS = Number(process.env.CHECK_TIMEOUT_MS ?? 30000);
const check = async (name, fn) => {
  process.stdout.write(`..  ${name}\r`);
  let timer;
  const timeout = new Promise((_, rej) => { timer = setTimeout(() => rej(new Error(`TIMEOUT after ${CHECK_TIMEOUT_MS} ms: ${name}`)), CHECK_TIMEOUT_MS); });
  try { await Promise.race([fn(), timeout]); } finally { clearTimeout(timer); }
  passed++; console.log(`ok  ${name}`);
};

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

await check("delegate + undelegate via staking precompile (power-cap hook path, ADR-0004)", async () => {
  const vals = await (await fetch(`${REST}/cosmos/staking/v1beta1/validators`)).json();
  const valoper = vals.validators[0].operator_address;
  const st = new ethers.Contract("0x0000000000000000000000000000000000000800", [
    "function delegate(address,string,uint256) returns (bool)",
    "function undelegate(address,string,uint256) returns (int64)",
    "function delegation(address,string) view returns (uint256 shares, (string denom, uint256 amount) balance)",
  ], wallet);
  const amt = ethers.parseEther("1000");
  const [, before] = await st.delegation(wallet.address, valoper);
  assert.equal((await (await st.delegate(wallet.address, valoper, amt, { gasLimit: 500000 })).wait()).status, 1);
  const [, mid] = await st.delegation(wallet.address, valoper);
  assert.equal(mid.amount - before.amount, amt);
  assert.equal((await (await st.undelegate(wallet.address, valoper, amt, { gasLimit: 500000 })).wait()).status, 1);
  const [, after] = await st.delegation(wallet.address, valoper);
  assert.equal(after.amount, before.amount);
});

await check("createValidator via staking precompile enforces the chain minimum self-bond (G-STAKE-1)", async () => {
  const st = new ethers.Contract("0x0000000000000000000000000000000000000800", [
    "function createValidator((string moniker,string identity,string website,string securityContact,string details) description, (uint256 rate,uint256 maxRate,uint256 maxChangeRate) commissionRates, uint256 minSelfDelegation, address validatorAddress, string pubkey, uint256 value) returns (bool)",
  ], wallet);
  const pubkey = Buffer.from(ethers.randomBytes(32)).toString("base64"); // ed25519 consensus key
  const desc = { moniker: "e2e", identity: "", website: "", securityContact: "", details: "" };
  const rates = { rate: ethers.parseEther("0.1"), maxRate: ethers.parseEther("0.2"), maxChangeRate: ethers.parseEther("0.01") };
  const value = ethers.parseEther("200000");
  // below the floor (1 NIRA) → reverts
  await assert.rejects(st.createValidator.staticCall(desc, rates, ethers.parseEther("1"), wallet.address, pubkey, value, { gasLimit: 2_000_000 }),
    e => /self-bond|min_self/i.test(String(e.shortMessage ?? e.message) + JSON.stringify(e.info ?? {})));
  // at the floor (100 000 NIRA) → would succeed
  assert.equal(await st.createValidator.staticCall(desc, rates, ethers.parseEther("100000"), wallet.address, pubkey, value, { gasLimit: 2_000_000 }), true);
});

await check("JSON-RPC: log, block and pending-tx filters; fee history; net/web3/txpool; WS newHeads (G-EVM-1)", async () => {
  const rpc = (m, p = []) => provider.send(m, p);
  assert.equal(await rpc("net_version"), EXPECTED_CHAIN_ID.toString());
  assert.equal(await rpc("net_listening"), true);
  assert.equal(await rpc("eth_syncing"), false);
  assert.match(await rpc("web3_clientVersion"), /Version/);
  const fh = await rpc("eth_feeHistory", ["0x4", "latest", [25, 75]]);
  assert.equal(fh.baseFeePerGas.length, 5);
  const tp = await rpc("txpool_status");
  assert.ok("pending" in tp && "queued" in tp);

  const addr = await counter.getAddress();
  const logFilter = await rpc("eth_newFilter", [{ address: addr, fromBlock: "latest" }]);
  const blockFilter = await rpc("eth_newBlockFilter");
  const pendingFilter = await rpc("eth_newPendingTransactionFilter");
  const tx = await counter.inc(1, { type: 2 });
  await tx.wait();
  const logs = await rpc("eth_getFilterChanges", [logFilter]);
  assert.ok(logs.some(l => l.transactionHash === tx.hash), "log filter returned the event");
  assert.ok((await rpc("eth_getFilterChanges", [blockFilter])).length >= 1, "block filter returned blocks");
  const pend = await rpc("eth_getFilterChanges", [pendingFilter]);
  assert.ok(Array.isArray(pend), "pending tx filter answers");
  assert.equal(await rpc("eth_uninstallFilter", [logFilter]), true);

  const ws = new ethers.WebSocketProvider(process.env.WS ?? "ws://127.0.0.1:8546");
  const head = await new Promise((res, rej) => {
    const t = setTimeout(() => rej(new Error("no newHeads within 15 s")), 15000);
    ws.on("block", n => { clearTimeout(t); res(n); });
  });
  assert.ok(head > 0);
  await ws.destroy();
});

await check("invalid txs are rejected at submission with a reason; failing execution is mined with status 0 (K7)", async () => {
  const to = ethers.Wallet.createRandom().address;
  const n = await provider.getTransactionCount(wallet.address, "pending");
  const gp = (await provider.getFeeData()).gasPrice * 2n;
  const reject = async (tx, re) => {
    const raw = await wallet.signTransaction({ chainId: EXPECTED_CHAIN_ID, type: 0, to, value: 1n, gasLimit: 21000, gasPrice: gp, nonce: n, ...tx });
    await assert.rejects(provider.send("eth_sendRawTransaction", [raw]), e => re.test(String(e.error?.message ?? e.message)));
  };
  await reject({ gasLimit: 1000 }, /intrinsic gas too low/);
  await reject({ nonce: 0 }, /nonce/);
  await reject({ value: 10n ** 40n }, /insufficient funds/);
  // passes CheckTx, reverts in execution → included, receipt status 0, nonce consumed (never silently dropped)
  const tx = await counter.inc(1000, { gasLimit: 100000 });
  const rc = await provider.waitForTransaction(tx.hash, 1, 30000);
  assert.equal(rc.status, 0);
  // cosmos/evm charges at least gasLimit × feemarket min_gas_multiplier (0.5): documented difference from Ethereum
  const mult = (await (await fetch(`${REST}/cosmos/evm/feemarket/v1/params`)).json()).params.min_gas_multiplier;
  assert.ok(rc.gasUsed >= 100000n * BigInt(Math.round(Number(mult) * 100)) / 100n);
  assert.equal(await provider.getTransactionCount(wallet.address, "latest"), n + 1);
});

console.log(`\n${passed} checks passed`);
process.exit(0);
