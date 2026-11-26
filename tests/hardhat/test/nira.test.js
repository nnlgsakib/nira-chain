// Hardhat 3 + ethers against Nira (G-TEST-3): deploy, call, events, custom-error
// revert, and cross-checks of precompile/JSON-RPC reads against Cosmos REST at
// the same block.
import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create("nira");
const STAKING = "0x0000000000000000000000000000000000000800";
const REST = process.env.REST ?? "http://127.0.0.1:1317";
const rest = async (path, height) =>
  (await fetch(REST + path, { headers: height ? { "x-cosmos-block-height": String(height) } : {} })).json();
const bech32Of = async (hex) => (await rest(`/cosmos/evm/vm/v1/cosmos_account/${hex}`)).cosmos_address;

describe("Nira via Hardhat", function () {
  this.timeout(120_000);
  let signer, counter;

  before(async () => {
    [signer] = await ethers.getSigners();
    counter = await ethers.deployContract("Counter");
    await counter.waitForDeployment();
  });

  it("reports chain id 93000", async () => {
    expect((await ethers.provider.getNetwork()).chainId).to.equal(93000n);
  });

  it("deploys a contract and reads its code", async () => {
    expect(await ethers.provider.getCode(await counter.getAddress())).to.not.equal("0x");
  });

  it("sends a tx that emits an event", async () => {
    const rc = await (await counter.inc(7)).wait();
    expect(rc.status).to.equal(1);
    const ev = rc.logs.map((l) => counter.interface.parseLog(l)).find((e) => e?.name === "Incremented");
    expect(ev.args.by).to.equal(signer.address);
    expect(await counter.count()).to.equal(ev.args.newCount);
  });

  it("decodes a custom-error revert", async () => {
    let err;
    try { await counter.inc.staticCall(101); } catch (e) { err = e; }
    expect(err, "inc(101) must revert").to.exist;
    expect(counter.interface.parseError(err.data).name).to.equal("TooLarge");
  });

  it("staking precompile delegation = Cosmos REST delegation (same block)", async () => {
    const valoper = (await rest("/cosmos/staking/v1beta1/validators")).validators[0].operator_address;
    const st = new ethers.Contract(STAKING,
      ["function delegation(address,string) view returns (uint256 shares,(string denom,uint256 amount) balance)"], ethers.provider);
    const h = await ethers.provider.getBlockNumber();
    const [, bal] = await st.delegation(signer.address, valoper, { blockTag: h });
    const d = await rest(`/cosmos/staking/v1beta1/validators/${valoper}/delegations/${await bech32Of(signer.address)}`, h);
    expect(bal.amount).to.equal(BigInt(d.delegation_response.balance.amount));
  });

  it("eth_getBalance = Cosmos bank balance (same block)", async () => {
    const h = await ethers.provider.getBlockNumber();
    const b = await rest(`/cosmos/bank/v1beta1/balances/${await bech32Of(signer.address)}/by_denom?denom=anira`, h);
    expect(await ethers.provider.getBalance(signer.address, h)).to.equal(BigInt(b.balance.amount));
  });
});
