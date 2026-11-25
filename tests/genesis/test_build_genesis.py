"""Unit tests for scripts/genesis/build_genesis.py validation rules.
Run: python3 -m unittest discover -s tests/genesis -p 'test_*.py'"""
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "scripts" / "genesis"))
import build_genesis as b  # noqa: E402

EXAMPLE = Path(__file__).resolve().parents[2] / "genesis" / "networks" / "mainnet.example.toml"


def net_from(text: str) -> b.Network:
    f = tempfile.NamedTemporaryFile("w", suffix=".toml", delete=False)
    f.write(text)
    f.close()
    return b.Network(Path(f.name))


class TestBuilder(unittest.TestCase):
    def setUp(self):
        self.base = EXAMPLE.read_text()

    def errors(self, text):
        return net_from(text).validate()[0]

    def test_example_is_valid(self):
        self.assertEqual(self.errors(self.base), [])

    def test_supply_must_balance_exactly(self):
        errs = self.errors(self.base.replace('total = "1000000000"', 'total = "1000000001"'))
        self.assertTrue(any("sum to" in e for e in errs), errs)

    def test_duplicate_address_rejected(self):
        t = self.base.replace("0x00000000000000000000000000000000000000c1", "0x00000000000000000000000000000000000000e1")
        self.assertTrue(any("already used" in e for e in self.errors(t)))

    def test_bech32_and_hex_are_the_same_address(self):
        a = "0x00000000000000000000000000000000000000e1"
        t = self.base.replace("0x00000000000000000000000000000000000000c1", b.acc(b.addr_bytes(a)))
        self.assertTrue(any("already used" in e for e in self.errors(t)))

    def test_single_validator_power_share(self):
        # shrink three validators to 10 NIRA: core-4 then holds ~99.99 % of voting power
        t = self.base
        for i in (1, 2, 3):
            t = t.replace(f'address = "0x000000000000000000000000000000000000010{i}"\nbalance = "1010000"\nself_bond = "1000000"',
                          f'address = "0x000000000000000000000000000000000000010{i}"\nbalance = "1010000"\nself_bond = "10"')
        errs = self.errors(t)
        self.assertTrue(any("core-4" in e and "of genesis voting power" in e for e in errs), errs)

    def test_self_bond_above_balance(self):
        t = self.base.replace('balance = "1010000"\nself_bond = "1000000"', 'balance = "1010000"\nself_bond = "2000000"', 1)
        self.assertTrue(any("> balance" in e for e in self.errors(t)))

    def test_min_validators(self):
        t = self.base.replace("min_validators = 4", "min_validators = 5")
        self.assertTrue(any("rules.min_validators" in e for e in self.errors(t)))

    def test_community_node_bond_and_onion(self):
        t = self.base.replace('bond = "1000"\nbalance = "10"', 'bond = "999"\nbalance = "11"', 1)
        self.assertTrue(any("below community.min_bond" in e for e in self.errors(t)))
        t = self.base.replace("376da11fe3ab3d0eaaddb418ccb49b5426d5c2504f526f7766580f6e45984e3b", "00" * 32)
        self.assertTrue(any("onion_tip" in e for e in self.errors(t)))

    def test_chain_id_must_bind_evm_chain_id(self):
        t = self.base.replace('chain_id = "nira_47201-1"', 'chain_id = "nira-1"')
        self.assertTrue(any("chain_id" in e for e in self.errors(t)))

    def test_amount_precision(self):
        self.assertEqual(b.nira_to_base("1.5"), 15 * 10**17)
        with self.assertRaises(ValueError):
            b.nira_to_base("0.0000000000000000001")
        self.assertEqual(b.convert_params({"x": "2 NIRA", "y": ["3 NIRA"]}), {"x": str(2 * 10**18), "y": [str(3 * 10**18)]})

    def test_module_address_matches_sdk(self):
        # verified against `nira-chaind q auth module-account community`
        self.assertEqual(b.acc(b.module_address("community")), "nira17d2wax0zhjrrecvaszuyxdf5wcu5a0p4hn5wfr")


if __name__ == "__main__":
    unittest.main()
