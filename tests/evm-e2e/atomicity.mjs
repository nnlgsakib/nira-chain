// G-SEC-4: x/community and staking precompile writes are atomic with the EVM
// call frame: a revert (top-level, or inside a caught sub-call) must leave no
// node, no bond escrow and no delegation behind.
import { readFileSync } from "node:fs";
import solc from "solc";
import { ethers } from "ethers";
import assert from "node:assert/strict";

const RPC = process.env.RPC ?? "http://127.0.0.1:8545";
const REST = process.env.COSMOS_REST ?? "http://127.0.0.1:1317";
const provider = new ethers.JsonRpcProvider(RPC, undefined, { cacheTimeout: -1 });
const wallet = new ethers.Wallet(process.env.KEY, provider);
const rest = async p => (await fetch(REST + p)).json();
let passed = 0;
const ok = name => { passed++; console.log(`ok  ${name}`); };

const out = JSON.parse(solc.compile(JSON.stringify({
  language: "Solidity", sources: { "Atomic.sol": { content: readFileSync(new URL("./Atomic.sol", import.meta.url), "utf8") } },
  settings: { evmVersion: "cancun", outputSelection: { "*": { "*": ["abi", "evm.bytecode.object"] } } },
})));
if (out.errors?.some(e => e.severity === "error")) throw new Error(JSON.stringify(out.errors));
const A = out.contracts["Atomic.sol"].Atomic;
const atomic = await (await new ethers.ContractFactory(A.abi, A.evm.bytecode.object, wallet).deploy()).waitForDeployment();
const addr = await atomic.getAddress();
const bond = ethers.parseEther("1000");
await (await wallet.sendTransaction({ to: addr, value: ethers.parseEther("3000") })).wait();

const { cosmos_address: bech } = await rest(`/cosmos/evm/vm/v1/cosmos_account/${addr}`);
// Escrowed bonds (the module balance also grows every block with the epoch reward budget)
const escrow = async () => BigInt((await rest(`/nira/community/v1/epoch`)).total_active_bond);
const bal = async a => BigInt((await rest(`/cosmos/bank/v1beta1/balances/${a}/by_denom?denom=anira`)).balance.amount);
const nodeExists = async () => (await fetch(`${REST}/nira/community/v1/nodes/${bech}`)).ok;
const valoper = (await rest(`/cosmos/staking/v1beta1/validators`)).validators[0].operator_address;
const tip = ethers.hexlify(ethers.randomBytes(32));

const c0 = await bal(bech), m0 = await escrow();

// 1. top-level revert
await assert.rejects((await atomic.registerThenRevert(bond, tip, { gasLimit: 2_000_000 })).wait());
assert.equal(await nodeExists(), false, "node must not exist after top-level revert");
assert.equal(await escrow(), m0, "community escrow (total_active_bond) unchanged");
ok("top-level revert after registerNode: no node, escrow unchanged");

// 2. caught inner revert; outer tx succeeds
const rc = await (await atomic.registerInTryCatch(bond, tip, valoper, { gasLimit: 3_000_000 })).wait();
assert.equal(rc.status, 1);
assert.equal(await atomic.afterCatch(), 1n, "outer frame committed");
assert.equal(await nodeExists(), false, "inner registerNode rolled back");
assert.equal(await escrow(), m0, "community escrow (total_active_bond) unchanged");
const dels = await rest(`/cosmos/staking/v1beta1/delegations/${bech}`);
assert.equal((dels.delegation_responses ?? []).length, 0, "inner delegate rolled back");
const c1 = await bal(bech);
assert.equal(c1, c0, "contract balance unchanged (bond and stake refunded by revert)");
const evmBal = await provider.getBalance(addr);
assert.equal(evmBal, c1, "EVM view == bank view after partial revert");
ok("caught inner revert: registerNode + delegate rolled back, outer state committed, balances consistent");

console.log(`\n${passed} checks passed`);
process.exit(0);
