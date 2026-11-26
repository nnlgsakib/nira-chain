// Package node is the desktop app's backend: it configures, runs and watches a
// lean Nira community node (a `nirad` process) and its work loop. It never
// re-implements chain logic: everything on-chain goes through `nirad` itself.
package node

import "fmt"

// Network is a chain the app can join.
type Network struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ChainID    string `json:"chain_id"`
	EVMChainID uint64 `json:"evm_chain_id"`
	Launched   bool   `json:"launched"`
	// RPC endpoints used for genesis, peers and state-sync trust. State sync needs
	// at least two that agree (same rule as scripts/trusted-checkpoint.py).
	RPC       []string `json:"rpc"`
	REST      string   `json:"rest"`
	Seeds     string   `json:"seeds"`
	StateSync bool     `json:"state_sync"`
	// P2PPort, when set, is the port peers are dialled on instead of the one the
	// RPC node reports (it differs behind Docker port publishing).
	P2PPort int    `json:"p2p_port,omitempty"`
	Note    string `json:"note"`
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
			ID: "docker", Name: "Docker testnet", ChainID: "nira_93000-88", EVMChainID: 93000, Launched: true,
			RPC: []string{"http://127.0.0.1:36657"}, REST: "http://127.0.0.1:31317", P2PPort: 36656,
			// 8 validators + 6 community nodes started by deploy/testnet/testnet.py up
			StateSync: false, Note: "Multi-node test network from deploy/testnet (Docker)",
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

// Ports of the app's own node. Defaults are offset from nirad's so the app can
// run next to a validator or a localnet on the same machine; a busy port is
// replaced by the next free one before the node starts (Manager.ensurePorts).
type Ports struct {
	P2P     int `json:"p2p"`
	RPC     int `json:"rpc"`
	REST    int `json:"rest"`
	GRPC    int `json:"grpc"`
	Metrics int `json:"metrics"`
}

// DefaultPorts are the first choice for a new node.
func DefaultPorts() Ports {
	return Ports{P2P: 26756, RPC: 26757, REST: 1417, GRPC: 9190, Metrics: 26760}
}

func (p Ports) valid() bool {
	return p.P2P > 0 && p.RPC > 0 && p.REST > 0 && p.GRPC > 0 && p.Metrics > 0
}

// RPCURL / RESTURL are the node's local endpoints.
func (p Ports) RPCURL() string  { return fmt.Sprintf("http://127.0.0.1:%d", p.RPC) }
func (p Ports) RESTURL() string { return fmt.Sprintf("http://127.0.0.1:%d", p.REST) }
func (p Ports) NodeArg() string { return fmt.Sprintf("tcp://127.0.0.1:%d", p.RPC) }
