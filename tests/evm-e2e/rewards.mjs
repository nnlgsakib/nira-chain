// Reward accounting e2e (deliverable 6): validator, commission, delegator and
// community-pool rewards checked to the last unit, on both interfaces.
//
// On a block with no transactions, every newly minted token must land in exactly
// one place: the community epoch budget (reward_share_bps), the community pool
// (community_tax), validator commission, or delegator rewards (F1, pro rata to
// stake). Reads are made at the same heights over REST and over the
// distribution precompile (eth_call at that block), and withdrawals are made
// both as Cosmos txs and as EVM precompile calls, with exact balance checks.
//
// Usage (running localnet): KEY=<dev1 eth key> node rewards.mjs
import { ethers } from "ethers";
import { execFileSync } from "node:child_process";

const REST = process.env.REST ?? "http://127.0.0.1:1317";
const RPC = process.env.EVM_RPC ?? "http://127.0.0.1:8545";
const BIN = process.env.BIN ?? "nirad";
const HOME = process.env.HOME_DIR ?? `${process.env.HOME}/.nira`;
const CHAIN_ID = process.env.CHAIN_ID ?? "nira_93000-1";
const DISTR = "0x0000000000000000000000000000000000000801";
const STAKING = "0x0000000000000000000000000000000000000800";
const E18 = 10n ** 18n;

const provider = new ethers.JsonRpcProvider(RPC);
const dev1 = new ethers.Wallet(process.env.KEY, provider);
const distr = new ethers.Contract(DISTR, [
  "function delegationRewards(address,string) view returns ((string denom,uint256 amount,uint8 precision)[])",
  "function validatorOutstandingRewards(string) view returns ((string denom,uint256 amount,uint8 precision)[])",
  "function validatorCommission(string) view returns ((string denom,uint256 amount,uint8 precision)[])",
  "function communityPool() view returns ((string denom,uint256 amount,uint8 precision)[])",
  "function withdrawDelegatorRewards(address,string) returns ((string denom,uint256 amount)[])",
  "event WithdrawDelegatorReward(address indexed delegatorAddress, address indexed validatorAddress, uint256 amount)",
], dev1);
const staking = new ethers.Contract(STAKING, ["function delegate(address,string,uint256) returns (bool)"], dev1);

let passed = 0;
const ok = (m) => { passed++; console.log(`ok  ${m}`); };
const fail = (m) => { console.error(`FAIL: ${m}`); process.exit(1); };
const eq = (a, b, m) => { if (a !== b) fail(`${m}: got ${a}, want ${b}`); };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function rest(path, height) {
  const r = await fetch(REST + path, { headers: height ? { "x-cosmos-block-height": String(height) } : {} });
  if (!r.ok) fail(`GET ${path} @${height}: ${r.status} ${await r.text()}`);
  return r.json();
}
// "12.345" (LegacyDec) → BigInt scaled by 1e18
const dec = (s) => { const [i, f = ""] = String(s).split("."); return BigInt(i) * E18 + BigInt((f + "0".repeat(18)).slice(0, 18)); };
const decOf = (coins) => dec((coins ?? []).find((c) => c.denom === "anira")?.amount ?? "0");
const intOf = (coins) => BigInt((coins ?? []).find((c) => c.denom === "anira")?.amount ?? "0");
// cosmos/evm v0.6.0 quirk (precompiles/common/types.go NewDecCoinsResponse): DecCoin.amount is the
// value TRUNCATED to whole anira, although precision says 18. Return it as whole anira.
const evmInt = (coins) => { const c = coins.find((x) => x.denom === "anira"); if (c && c.precision !== 18n) fail(`precision ${c.precision}`); return c ? c.amount : 0n; };
const height = async () => Number((await rest("/cosmos/base/tendermint/v1beta1/blocks/latest")).block.header.height);
const waitHeight = async (h) => { while ((await height()) < h) await sleep(300); };
const bank = async (addr, h) => BigInt((await rest(`/cosmos/bank/v1beta1/balances/${addr}/by_denom?denom=anira`, h)).balance.amount);
const hexOf = (bech) => ethers.getAddress("0x" + Buffer.from(bech32Decode(bech)).toString("hex"));

function bech32Decode(s) {
  const CH = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
  const data = [...s.slice(s.lastIndexOf("1") + 1, -6)].map((c) => CH.indexOf(c));
  let acc = 0, bits = 0; const out = [];
  for (const v of data) { acc = (acc << 5) | v; bits += 5; while (bits >= 8) { bits -= 8; out.push((acc >> bits) & 0xff); } }
  return out;
}

// Cosmos tx through the CLI; returns the included tx result
async function cosmosTx(args) {
  const bf = Number((await rest("/cosmos/evm/feemarket/v1/base_fee")).base_fee);
  const out = JSON.parse(execFileSync(BIN, ["tx", ...args, "--chain-id", CHAIN_ID, "--keyring-backend", "test", "--home", HOME,
    "--gas", "auto", "--gas-adjustment", "1.5", "--gas-prices", `${Math.max(1, Math.ceil(bf * 2))}anira`, "-y", "--output", "json"],
    { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }));
  if (out.code !== 0) fail(`tx ${args.join(" ")}: ${out.raw_log}`);
  for (let i = 0; i < 30; i++) {
    await sleep(1000);
    try {
      const r = JSON.parse(execFileSync(BIN, ["q", "tx", out.txhash, "--output", "json", "--home", HOME], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }));
      if (r.code !== 0) fail(`tx ${args.join(" ")}: ${r.raw_log}`);
      return r;
    } catch { /* not yet included */ }
  }
  fail(`tx ${out.txhash} not included`);
}
const eventAmount = (res, type, extra = () => true) => {
  let total = 0n;
  for (const e of res.events.filter((e) => e.type === type && extra(e))) {
    const a = e.attributes.find((x) => x.key === "amount")?.value ?? "";
    const m = a.match(/^(\d+)anira$/);
    if (m) total += BigInt(m[1]);
  }
  return total;
};
const feeOf = (res) => intOf(res.tx.auth_info.fee.amount);

const keyAddr = (name) => execFileSync(BIN, ["keys", "show", name, "-a", "--keyring-backend", "test", "--home", HOME], { encoding: "utf8" }).trim();
const VALOPER = (await rest("/cosmos/staking/v1beta1/validators")).validators[0].operator_address;
const SELF = keyAddr("validator");
const DEV1 = keyAddr("dev1");
const DEV2 = keyAddr("dev2");
if (hexOf(DEV1) !== dev1.address) fail(`KEY is not dev1 (${dev1.address} vs ${hexOf(DEV1)})`);
const FEE_COLLECTOR = (await rest("/cosmos/auth/v1beta1/module_accounts/fee_collector")).account.base_account.address;

// --- setup: one delegation per interface --------------------------------------
await cosmosTx(["staking", "delegate", VALOPER, `${1000n * E18}anira`, "--from", "dev2"]);
const dtx = await staking.delegate(dev1.address, VALOPER, 2000n * E18, { gasLimit: 500_000 });
if ((await dtx.wait()).status !== 1) fail("precompile delegate reverted");
await waitHeight((await height()) + 2);
ok("setup: dev2 delegated 1 000 NIRA (Cosmos tx), dev1 delegated 2 000 NIRA (staking precompile)");

// --- 1. exact accounting of one quiet block -----------------------------------
const p = {
  tax: dec((await rest("/cosmos/distribution/v1beta1/params")).params.community_tax),
  bpy: BigInt((await rest("/cosmos/mint/v1beta1/params")).params.blocks_per_year),
  share: BigInt((await rest("/nira/community/v1/params")).params.reward_share_bps),
};
const rate = dec((await rest(`/cosmos/staking/v1beta1/validators/${VALOPER}`)).validator.commission.commission_rates.rate);
// every delegator of the validator (other suites may have delegated too), named where known
const named = { [SELF]: "validator", [DEV1]: "dev1", [DEV2]: "dev2" };
const dels = {};
for (const d of (await rest(`/cosmos/staking/v1beta1/validators/${VALOPER}/delegations?pagination.limit=1000`)).delegation_responses)
  dels[named[d.delegation.delegator_address] ?? d.delegation.delegator_address] = d.delegation.delegator_address;

async function snapshot(h) {
  const s = {
    supply: BigInt((await rest("/cosmos/bank/v1beta1/supply/by_denom?denom=anira", h)).amount.amount),
    provisions: dec((await rest("/cosmos/mint/v1beta1/annual_provisions", h)).annual_provisions),
    fc: await bank(FEE_COLLECTOR, h),
    epoch: (await rest("/nira/community/v1/epoch", h)).epoch,
    out: decOf((await rest(`/cosmos/distribution/v1beta1/validators/${VALOPER}/outstanding_rewards`, h)).rewards.rewards),
    comm: decOf((await rest(`/cosmos/distribution/v1beta1/validators/${VALOPER}/commission`, h)).commission.commission),
    pool: decOf((await rest("/cosmos/distribution/v1beta1/community_pool", h)).pool),
    tokens: BigInt((await rest(`/cosmos/staking/v1beta1/validators/${VALOPER}`, h)).validator.tokens),
    txs: (await rest(`/cosmos/base/tendermint/v1beta1/blocks/${h}`)).block.data.txs.length,
    rew: {}, stake: {},
  };
  for (const [n, a] of Object.entries(dels)) {
    s.rew[n] = decOf((await rest(`/cosmos/distribution/v1beta1/delegators/${a}/rewards/${VALOPER}`, h)).rewards);
    s.stake[n] = BigInt((await rest(`/cosmos/staking/v1beta1/validators/${VALOPER}/delegations/${a}`, h)).delegation_response.balance.amount);
  }
  return s;
}

let H, a, b;
for (let tries = 0; ; tries++) {
  if (tries > 60) fail("no quiet block pair found");
  const top = await height();
  H = top - 1;
  a = await snapshot(H);
  b = await snapshot(H + 1);
  // block H carries no txs (so H+1 distributes only new issuance), and no epoch boundary in between
  if (a.txs === 0 && a.fc === 0n && a.epoch.number === b.epoch.number) break;
  await waitHeight(top + 1);
}
const minted = b.supply - a.supply;
eq(minted, b.provisions / p.bpy / E18, `minted at ${H + 1} = truncate(annual_provisions / blocks_per_year)`);
const budget = BigInt(b.epoch.budget) - BigInt(a.epoch.budget);
eq(budget, (minted * p.share) / 10000n, "community budget share = floor(minted × reward_share_bps / 10 000)");
const toDistr = (minted - budget) * E18;
const valTok = toDistr - (toDistr * p.tax) / E18;
eq(b.out - a.out, valTok, "validator outstanding rewards = distributed × (1 − community_tax)");
eq(b.pool - a.pool, toDistr - valTok, "community pool += distributed × community_tax");
const comm = (valTok * rate) / E18;
eq(b.comm - a.comm, comm, "commission = validator rewards × commission rate");
ok(`block ${H + 1}: minted ${minted} anira = community budget ${budget} + community tax + commission + delegator rewards, exact to 1e-18 anira`);

const shared = valTok - comm;
let sum = 0n, tol = 0n;
const total = a.tokens;
for (const n of Object.keys(dels)) {
  const got = b.rew[n] - a.rew[n];
  const want = (shared * a.stake[n]) / total;
  const t = a.stake[n] + 10n; // F1 keeps reward-per-token to 18 decimals: ≤ stake × 1e-18 anira (= stake in 1e-18 units)
  if (got > want + t || got < want - t) fail(`${n} reward ${got} vs pro-rata ${want} (±${t})`);
  sum += got; tol += t;
}
// F1 rounds the per-token ratio to nearest (LegacyDec), so the total may land slightly above or below
if (sum > shared + tol || sum < shared - tol) fail(`Σ delegator rewards ${sum} vs shared ${shared} (±${tol})`);
ok(`delegator rewards pro rata to stake for all ${Object.keys(dels).length} delegators (validator ${a.stake.validator / E18}, dev1 ${a.stake.dev1 / E18}, dev2 ${a.stake.dev2 / E18} NIRA, …); Σ = validator rewards − commission`);

// --- 2. EVM precompile reads the same numbers at the same height ---------------
const at = { blockTag: H + 1 };
eq(evmInt(await distr.validatorOutstandingRewards(VALOPER, at)), b.out / E18, "precompile validatorOutstandingRewards");
eq(evmInt(await distr.validatorCommission(VALOPER, at)), b.comm / E18, "precompile validatorCommission");
eq(evmInt(await distr.communityPool(at)), b.pool / E18, "precompile communityPool");
for (const [n, addr] of Object.entries(dels)) eq(evmInt(await distr.delegationRewards(hexOf(addr), VALOPER, at)), b.rew[n] / E18, `precompile delegationRewards ${n}`);
ok(`distribution precompile at block ${H + 1} = REST truncated to whole anira (outstanding, commission, community pool, 3 delegators)`);

// --- 3. withdraw as a Cosmos tx (dev2) -----------------------------------------
{
  const r = await cosmosTx(["distribution", "withdraw-rewards", VALOPER, "--from", "dev2"]);
  const h = Number(r.height);
  const got = eventAmount(r, "withdraw_rewards");
  if (got <= 0n) fail("no withdraw_rewards amount");
  eq(await bank(DEV2, h) - await bank(DEV2, h - 1), got - feeOf(r), "dev2 balance change = withdrawn − fee");
  const before = decOf((await rest(`/cosmos/distribution/v1beta1/delegators/${DEV2}/rewards/${VALOPER}`, h - 1)).rewards);
  if (got < before / E18) fail(`withdrew ${got} < accrued ${before / E18} at ${h - 1}`);
  eq(decOf((await rest(`/cosmos/distribution/v1beta1/delegators/${DEV2}/rewards/${VALOPER}`, h)).rewards), 0n, "dev2 rewards after withdraw");
  eq(await provider.getBalance(hexOf(DEV2), h), await bank(DEV2, h), "dev2 eth_getBalance = bank after withdraw");
  ok(`Cosmos withdraw-rewards (dev2): +${got} anira = accrued rewards; balance delta = withdrawn − fee; rewards reset to 0; EVM balance agrees`);
}

// --- 4. withdraw through the distribution precompile (dev1) --------------------
{
  const tx = await distr.withdrawDelegatorRewards(dev1.address, VALOPER, { gasLimit: 500_000 });
  const rc = await tx.wait();
  if (rc.status !== 1) fail("precompile withdraw reverted");
  const ev = rc.logs.map((l) => { try { return distr.interface.parseLog(l); } catch { return null; } }).find((x) => x?.name === "WithdrawDelegatorReward");
  if (!ev) fail("no WithdrawDelegatorReward log");
  const got = ev.args.amount, h = rc.blockNumber;
  const gasCost = rc.gasUsed * rc.gasPrice;
  eq(await bank(DEV1, h) - await bank(DEV1, h - 1), got - gasCost, "dev1 balance change = withdrawn − gas cost");
  const before = decOf((await rest(`/cosmos/distribution/v1beta1/delegators/${DEV1}/rewards/${VALOPER}`, h - 1)).rewards);
  if (got < before / E18) fail(`withdrew ${got} < accrued ${before / E18}`);
  eq(decOf((await rest(`/cosmos/distribution/v1beta1/delegators/${DEV1}/rewards/${VALOPER}`, h)).rewards), 0n, "dev1 REST rewards after precompile withdraw");
  ok(`precompile withdrawDelegatorRewards (dev1): +${got} anira; bank delta = withdrawn − gasUsed × gasPrice; REST shows rewards reset`);
}

// --- 5. commission withdrawal (validator operator, Cosmos) ---------------------
{
  const r = await cosmosTx(["distribution", "withdraw-rewards", VALOPER, "--commission", "--from", "validator"]);
  const h = Number(r.height);
  const c = eventAmount(r, "withdraw_commission");
  const w = eventAmount(r, "withdraw_rewards");
  if (c <= 0n) fail("no withdraw_commission amount");
  eq(await bank(SELF, h) - await bank(SELF, h - 1), c + w - feeOf(r), "operator balance change = commission + self rewards − fee");
  const left = decOf((await rest(`/cosmos/distribution/v1beta1/validators/${VALOPER}/commission`, h)).commission.commission);
  if (left >= E18) fail(`commission left after withdraw ${left} ≥ 1 anira`);
  ok(`commission withdraw: +${c} anira commission + ${w} self rewards; only the sub-unit remainder (< 1 anira) stays accrued`);
}

console.log(`\n${passed} reward checks passed`);
