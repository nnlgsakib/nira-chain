// Ethereum JSON-RPC conformance checks (G-TEST-3) against a running localnet.
// Not the official execution-apis test suite (that needs its own fixture chain):
// this checks, for the methods wallets and tooling rely on, that results follow
// the execution-apis encoding rules (QUANTITY = 0x-hex without leading zeros,
// DATA = even-length 0x-hex of the right size), that blocks, transactions and
// receipts carry the required fields, and that errors use JSON-RPC codes.
//
// Usage: KEY=<dev1 eth key> node rpc-conformance.mjs
import { ethers } from "ethers";

const RPC = process.env.EVM_RPC ?? "http://127.0.0.1:8545";
let passed = 0, id = 0;
const ok = (m) => { passed++; console.log(`ok  ${m}`); };
const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1); };

async function raw(method, params = []) {
  const r = await fetch(RPC, { method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: ++id, method, params }) });
  const j = await r.json();
  if (j.jsonrpc !== "2.0" || j.id !== id) fail(`${method}: bad envelope ${JSON.stringify(j)}`);
  return j;
}
async function call(method, params = []) {
  const j = await raw(method, params);
  if (j.error) fail(`${method}: ${JSON.stringify(j.error)}`);
  return j.result;
}

const QTY = /^0x(0|[1-9a-f][0-9a-f]*)$/;
const data = (n) => new RegExp(`^0x[0-9a-f]{${n * 2}}$`);
const DATA = /^0x([0-9a-f]{2})*$/;
const check = (v, re, what) => { if (typeof v !== "string" || !re.test(v)) fail(`${what}: ${JSON.stringify(v)} does not match ${re}`); };
const fields = (obj, spec, what) => {
  for (const [k, re] of Object.entries(spec)) {
    if (!(k in obj)) fail(`${what}: missing ${k}`);
    if (re && obj[k] !== null) check(obj[k], re, `${what}.${k}`);
  }
};

const wallet = new ethers.Wallet(process.env.KEY, new ethers.JsonRpcProvider(RPC));

// 1. identity and scalar methods
check(await call("eth_chainId"), QTY, "eth_chainId");
if (BigInt(await call("eth_chainId")) !== 47201n) fail("chain id != 47201");
if ((await call("net_version")) !== "47201") fail("net_version != \"47201\"");
if (typeof (await call("web3_clientVersion")) !== "string") fail("web3_clientVersion");
for (const m of ["eth_blockNumber", "eth_gasPrice", "eth_maxPriorityFeePerGas"]) check(await call(m), QTY, m);
const syncing = await call("eth_syncing");
if (syncing !== false && typeof syncing !== "object") fail(`eth_syncing ${syncing}`);
check(await call("eth_getBalance", [wallet.address, "latest"]), QTY, "eth_getBalance");
check(await call("eth_getTransactionCount", [wallet.address, "latest"]), QTY, "eth_getTransactionCount");
check(await call("eth_getCode", [wallet.address, "latest"]), DATA, "eth_getCode");
check(await call("eth_getStorageAt", [wallet.address, "0x0", "latest"]), data(32), "eth_getStorageAt");
ok("identity + scalars: eth_chainId/net_version 47201, QUANTITY/DATA encodings (blockNumber, gasPrice, maxPriorityFeePerGas, balance, nonce, code, storage), eth_syncing");

// 2. a real tx, then block / tx / receipt objects
const tx = await wallet.sendTransaction({ to: ethers.Wallet.createRandom().address, value: 1n, type: 2 });
const rc = await tx.wait();
const HASH = data(32), ADDR = data(20);
const block = await call("eth_getBlockByNumber", [ethers.toQuantity(rc.blockNumber), false]);
fields(block, { number: QTY, hash: HASH, parentHash: HASH, nonce: data(8), sha3Uncles: HASH, logsBloom: data(256), transactionsRoot: HASH,
  stateRoot: HASH, receiptsRoot: HASH, miner: ADDR, difficulty: QTY, extraData: DATA, size: QTY, gasLimit: QTY, gasUsed: QTY, timestamp: QTY,
  transactions: null, uncles: null, baseFeePerGas: QTY }, "block");
if (!block.transactions.includes(tx.hash)) fail("block.transactions lacks the tx hash");
const byHash = await call("eth_getBlockByHash", [block.hash, true]);
if (byHash.number !== block.number || typeof byHash.transactions[0] !== "object") fail("eth_getBlockByHash(full) mismatch");
check(await call("eth_getBlockTransactionCountByNumber", [block.number]), QTY, "eth_getBlockTransactionCountByNumber");
ok(`block objects: all required header fields well-formed (block ${rc.blockNumber}); by-number = by-hash; full-tx mode`);

const t = await call("eth_getTransactionByHash", [tx.hash]);
fields(t, { hash: HASH, nonce: QTY, blockHash: HASH, blockNumber: QTY, transactionIndex: QTY, from: ADDR, to: ADDR, value: QTY, gas: QTY,
  input: DATA, type: QTY, maxFeePerGas: QTY, maxPriorityFeePerGas: QTY, chainId: QTY, v: QTY, r: QTY, s: QTY }, "tx");
if (t.type !== "0x2") fail(`tx type ${t.type}`);
const r = await call("eth_getTransactionReceipt", [tx.hash]);
fields(r, { transactionHash: HASH, transactionIndex: QTY, blockHash: HASH, blockNumber: QTY, from: ADDR, to: null, cumulativeGasUsed: QTY,
  gasUsed: QTY, effectiveGasPrice: QTY, contractAddress: null, logs: null, logsBloom: data(256), type: QTY, status: QTY }, "receipt");
if (r.status !== "0x1" || r.blockHash !== block.hash) fail("receipt status/blockHash");
const byIdx = await call("eth_getTransactionByBlockNumberAndIndex", [block.number, t.transactionIndex]);
if (byIdx.hash !== tx.hash) fail("eth_getTransactionByBlockNumberAndIndex");
ok("EIP-1559 tx + receipt objects: required fields well-formed, status 0x1, lookups by hash and by block+index agree");

// 3. call / estimate / access list / fee history / logs / filters
check(await call("eth_call", [{ to: wallet.address, data: "0x" }, "latest"]), DATA, "eth_call");
check(await call("eth_estimateGas", [{ from: wallet.address, to: wallet.address, value: "0x1" }]), QTY, "eth_estimateGas");
const fh = await call("eth_feeHistory", ["0x4", "latest", [25, 75]]);
fields(fh, { oldestBlock: QTY, baseFeePerGas: null, gasUsedRatio: null, reward: null }, "feeHistory");
if (fh.baseFeePerGas.length !== 5 || fh.reward.length !== 4 || fh.reward[0].length !== 2) fail(`feeHistory shapes ${JSON.stringify(fh)}`);
fh.baseFeePerGas.forEach((x) => check(x, QTY, "feeHistory.baseFeePerGas"));
const logs = await call("eth_getLogs", [{ fromBlock: block.number, toBlock: block.number }]);
if (!Array.isArray(logs)) fail("eth_getLogs not an array");
const fid = await call("eth_newBlockFilter");
check(fid, QTY, "eth_newBlockFilter id");
if (!Array.isArray(await call("eth_getFilterChanges", [fid]))) fail("eth_getFilterChanges");
if ((await call("eth_uninstallFilter", [fid])) !== true) fail("eth_uninstallFilter");
ok("eth_call, eth_estimateGas, eth_feeHistory (4 blocks × 2 percentiles), eth_getLogs, block filter lifecycle");

// 4. errors use JSON-RPC 2.0 codes and never a result
const unknown = await raw("eth_noSuchMethod");
if (!unknown.error || unknown.error.code !== -32601 || "result" in unknown) fail(`unknown method → ${JSON.stringify(unknown)}`);
const bad = await raw("eth_getBalance", ["not-an-address", "latest"]);
if (!bad.error || "result" in bad || typeof bad.error.code !== "number") fail(`invalid params → ${JSON.stringify(bad)}`);
const missing = await call("eth_getTransactionByHash", ["0x" + "ab".repeat(32)]);
if (missing !== null) fail("unknown tx hash must return null");
const rev = await raw("eth_call", [{ to: "0x0000000000000000000000000000000000000800", data: "0xdeadbeef" }, "latest"]);
if (!rev.error || rev.error.code !== 3 && rev.error.code !== -32000) fail(`revert → ${JSON.stringify(rev)}`);
ok(`errors: unknown method -32601, invalid params → error ${bad.error.code}, unknown tx → null, reverting eth_call → error ${rev.error.code}`);

console.log(`\n${passed} JSON-RPC conformance checks passed`);
