// EVM-side e2e for x/community via the precompile at 0x…0900 (ADR-0006).
// An Ethereum key does the whole community-node flow through Solidity ABI
// calls; every step is cross-checked against the Cosmos REST interface.
// Usage: KEY=<hex> node community.mjs   (localnet with 20-block epochs)
import { readFileSync } from "node:fs";
import { createHash, randomBytes } from "node:crypto";
import { ethers } from "ethers";
import assert from "node:assert/strict";

const RPC = process.env.RPC ?? "http://127.0.0.1:8545";
const REST = process.env.COSMOS_REST ?? "http://127.0.0.1:1317";
const CMT = process.env.CMT ?? "http://127.0.0.1:26657";
const provider = new ethers.JsonRpcProvider(RPC, undefined, { cacheTimeout: -1 });
const wallet = new ethers.Wallet(process.env.KEY, provider);
const abi = JSON.parse(readFileSync(new URL("../../precompiles/community/abi.json", import.meta.url))).abi;
const C = new ethers.Contract("0x0000000000000000000000000000000000000900", abi, wallet);
const sha = (...parts) => createHash("sha256").update(Buffer.concat(parts.map(p => Buffer.from(p)))).digest();
const rest = async p => { const r = await fetch(REST + p); if (!r.ok) throw new Error(`${p}: ${r.status}`); return r.json(); };
const sleep = ms => new Promise(r => setTimeout(r, ms));
let passed = 0;
const check = async (name, fn, ms = 120000) => {
  let t; const to = new Promise((_, rej) => { t = setTimeout(() => rej(new Error(`TIMEOUT: ${name}`)), ms); });
  try { await Promise.race([fn(), to]); } finally { clearTimeout(t); }
  passed++; console.log(`ok  ${name}`);
};

// Local onion (secret never leaves this process)
const LEN = 10000;
const secret = randomBytes(32);
const hashN = (v, n) => { for (let i = 0; i < n; i++) v = sha(v); return v; };
const tip = hashN(secret, LEN);
const layer = (e, base) => hashN(secret, LEN - Number(e - base + 1n));

const { cosmos_address: bech } = await rest(`/cosmos/evm/vm/v1/cosmos_account/${wallet.address}`);
const height = async () => Number((await (await fetch(`${CMT}/status`)).json()).result.sync_info.latest_block_height);
const gasOpts = { gasLimit: 1_500_000 };

await check("registerNode via precompile (bond escrow, NodeRegistered event)", async () => {
  const before = await provider.getBalance(wallet.address);
  const bond = ethers.parseEther("1000");
  const tx = await C.registerNode("evm-node", bond, tip, LEN, gasOpts);
  const rc = await tx.wait();
  assert.equal(rc.status, 1);
  const ev = rc.logs.map(l => { try { return C.interface.parseLog(l); } catch { return null; } }).find(e => e?.name === "NodeRegistered");
  assert.ok(ev, "NodeRegistered event");
  assert.equal(ev.args.operator, wallet.address);
  assert.equal(ev.args.bond, bond);
  const after = await provider.getBalance(wallet.address);
  assert.ok(before - after >= bond, "bond left the EVM balance");
});

let node;
await check("Cosmos REST sees the EVM-registered node; precompile view agrees", async () => {
  const r = await rest(`/nira/community/v1/nodes/${bech}`);
  assert.equal(r.node.status, "NODE_STATUS_ACTIVE");
  assert.equal(r.node.bond, ethers.parseEther("1000").toString());
  node = await C.getNode(wallet.address);
  assert.equal(node.status, 1n);
  assert.equal(node.bond, ethers.parseEther("1000"));
  assert.equal(node.onionBaseEpoch.toString(), r.node.onion_base_epoch);
});

let ep;
await check("wait for eligibility; currentEpoch/getTasks agree with Cosmos REST", async () => {
  for (;;) { ep = await C.currentEpoch(); if (ep.number >= node.onionBaseEpoch) break; await sleep(2000); }
  await sleep(2000);
  ep = await C.currentEpoch();
  const re = await rest(`/nira/community/v1/epoch`);
  assert.equal(ep.number.toString(), re.epoch.number);
  assert.equal(ethers.hexlify(ep.seed), "0x" + Buffer.from(re.epoch.seed, "base64").toString("hex"));
  const tasks = await C.getTasks(wallet.address);
  const rt = await rest(`/nira/community/v1/tasks/${bech}`);
  assert.equal(tasks.length, rt.tasks.length);
  tasks.forEach((t, i) => {
    assert.equal(t.store, rt.tasks[i].store);
    assert.equal(t.height.toString(), rt.tasks[i].height);
    assert.equal(ethers.hexlify(t.key), "0x" + Buffer.from(rt.tasks[i].key, "base64").toString("hex"));
  });
});

// MerkleProof{repeated CommitmentProof proofs = 1}: each ProofOp.data is a CommitmentProof.
const merkleProof = ops => Buffer.concat(ops.map(op => {
  const d = Buffer.from(op.data, "base64");
  const len = []; let n = d.length; do { let b = n & 0x7f; n >>= 7; if (n) b |= 0x80; len.push(b); } while (n);
  return Buffer.concat([Buffer.from([0x0a, ...len]), d]);
}));
const proofFor = async t => {
  const q = new URLSearchParams({ path: `"/store/${t.store}/key"`, data: ethers.hexlify(t.key), height: t.height.toString(), prove: "true" });
  const r = (await (await fetch(`${CMT}/abci_query?${q}`)).json()).result.response;
  return { value: r.value ? Buffer.from(r.value, "base64") : Buffer.alloc(0), proof: merkleProof(r.proofOps.ops) };
};

let reveal, provenEpoch;
await check("selection computed locally from the onion matches the chain rule", async () => {
  const params = (await rest(`/nira/community/v1/params`)).params;
  reveal = layer(ep.number, node.onionBaseEpoch);
  const ticket = BigInt("0x" + sha(Buffer.from("nira/comm/ticket"), ethers.getBytes(ep.seed), reveal, ethers.getBytes(wallet.address)).toString("hex"));
  const selected = ticket * ep.eligibleBond < (1n << 256n) * BigInt(params.target_committee) * node.bond;
  assert.ok(selected, "localnet target_committee is large: node must be selected");
});

await check("tampered proof via precompile reverts", async () => {
  const [t] = await C.getTasks(wallet.address);
  const { value, proof } = await proofFor(t);
  proof[proof.length - 5] ^= 0xff;
  await assert.rejects(C.submitTaskProof.staticCall(ep.number, reveal, t.index, t.store, t.height, value, proof, gasOpts));
});

await check("submitTaskProof x2 via precompile with real ICS-23 proofs; Cosmos sees the units", async () => {
  const tasks = await C.getTasks(wallet.address);
  for (const t of tasks) {
    const { value, proof } = await proofFor(t);
    const rc = await (await C.submitTaskProof(ep.number, reveal, t.index, t.store, t.height, value, proof, gasOpts)).wait();
    assert.equal(rc.status, 1);
    assert.ok(rc.logs.some(l => { try { return C.interface.parseLog(l)?.name === "TaskProven"; } catch { return false; } }));
  }
  provenEpoch = ep.number;
  const r = await rest(`/nira/community/v1/nodes/${bech}`);
  assert.equal(r.units_this_epoch, tasks.length);
  assert.equal((await C.getNode(wallet.address)).unitsThisEpoch, BigInt(tasks.length));
});

await check("duplicate proof via precompile reverts (no double pay)", async () => {
  const [t] = await C.getTasks(wallet.address);
  const { value, proof } = await proofFor(t);
  await assert.rejects(C.submitTaskProof.staticCall(ep.number, reveal, t.index, t.store, t.height, value, proof, gasOpts));
});

await check("epoch settlement pays the EVM operator (community_reward event)", async () => {
  const settle = Number(ep.nextEpochHeight);
  while (await height() <= settle) await sleep(1000);
  const res = (await (await fetch(`${CMT}/block_results?height=${settle}`)).json()).result;
  const ev = (res.finalize_block_events ?? []).find(e => e.type === "community_reward" &&
    e.attributes.some(a => a.key === "operator" && a.value === bech));
  assert.ok(ev, `no community_reward for ${bech} at ${settle} (epoch ${provenEpoch})`);
  const amt = BigInt(ev.attributes.find(a => a.key === "amount").value);
  assert.ok(amt > 0n);
  // G-COMM-3: cumulative reward tracking, same on both interfaces
  assert.equal((await C.getNode(wallet.address)).rewardsTotal, amt);
  assert.equal((await rest(`/nira/community/v1/nodes/${bech}`)).node.rewards_total, amt.toString());
  console.log(`    paid ${ethers.formatEther(amt)} NIRA`);
}, 180000);

await check("deregisterNode via precompile → UNBONDING on Cosmos side", async () => {
  const rc = await (await C.deregisterNode(gasOpts)).wait();
  assert.equal(rc.status, 1);
  const r = await rest(`/nira/community/v1/nodes/${bech}`);
  assert.equal(r.node.status, "NODE_STATUS_UNBONDING");
  assert.equal((await C.getNode(wallet.address)).status, 2n);
});

console.log(`\n${passed} checks passed`);
process.exit(0);
