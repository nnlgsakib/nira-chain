// Package node is the desktop app's backend: it configures, runs and watches a
// lean Nira community node (a `nirad` process) and its work loop. It never
// re-implements chain logic: everything on-chain goes through `nirad` itself.
package node

// Network is a chain the app can join.
type Network struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ChainID    string   `json:"chain_id"`
	EVMChainID uint64   `json:"evm_chain_id"`
	Launched   bool     `json:"launched"`
	// RPC endpoints used for genesis, peers and state-sync trust. State sync needs
	// at least two that agree (same rule as scripts/trusted-checkpoint.py).
	RPC       []string `json:"rpc"`
	REST      string   `json:"rest"`
	Seeds     string   `json:"seeds"`
	StateSync bool     `json:"state_sync"`
	Note      string   `json:"note"`
}

// Networks lists the presets. Mainnet and testnet are placeholders until their
// genesis and public endpoints exist; the app shows them as not launched.
func Networks() []Network {
	return []Network{
		{
			ID: "mainnet", Name: "Mainnet", ChainID: "nira_91000-1", EVMChainID: 91000,
			StateSync: true, Note: "Not launched yet",
		},
		{
			ID: "testnet", Name: "Testnet", ChainID: "nira_92000-1", EVMChainID: 92000,
			StateSync: true, Note: "Not launched yet",
		},
		{
			ID: "local", Name: "Local network", ChainID: "nira_93000-1", EVMChainID: 93000, Launched: true,
			RPC: []string{"http://127.0.0.1:26657"}, REST: "http://127.0.0.1:1317",
			// a local dev chain keeps no snapshots: block sync from its validator
			StateSync: false, Note: "Development chain started by scripts/localnet.sh",
		},
	}
}

// NetworkByID returns the preset with that id.
func NetworkByID(id string) (Network, bool) {
	for _, n := range Networks() {
		if n.ID == id {
			return n, true
		}
	}
	return Network{}, false
}

// Ports of the app's own node. They are offset from the defaults so the app can
// run next to a validator or a localnet on the same machine.
const (
	PortP2P        = 26756
	PortRPC        = 26757
	PortREST       = 1417
	PortGRPC       = 9190
	PortPrometheus = 26760
)
