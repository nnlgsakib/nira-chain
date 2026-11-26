package node

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
)

// Status is everything the dashboard shows. Every field comes from the local
// node (or the local processes); nothing is estimated from tokenomics.
type Status struct {
	UpdatedAt   time.Time `json:"updated_at"`
	Network     string    `json:"network"`
	ChainID     string    `json:"chain_id"`
	Initialized bool      `json:"initialized"`
	HasKey      bool      `json:"has_key"`
	Locked      bool      `json:"locked"`
	NiradFound  bool      `json:"nirad_found"`

	NodeRunning   bool `json:"node_running"`
	WorkerRunning bool `json:"worker_running"`

	// Sync
	Reachable  bool      `json:"reachable"`
	Height     int64     `json:"height"`
	BlockTime  time.Time `json:"block_time"`
	CatchingUp bool      `json:"catching_up"`
	Peers      int       `json:"peers"`

	// Account and community node
	Operator     string `json:"operator"`
	Balance      string `json:"balance"`     // anira
	NodeStatus   string `json:"node_status"` // "", ACTIVE, UNBONDING
	Bond         string `json:"bond"`
	UnitsTotal   string `json:"units_total"`
	RewardsTotal string `json:"rewards_total"`
	SlashedTotal string `json:"slashed_total"`
	Epoch        string `json:"epoch"`
	NextEpoch    string `json:"next_epoch_height"`
	MinBond      string `json:"min_bond"`
	UnbondAt     string `json:"unbond_complete_height"`

	HasOnion  bool  `json:"has_onion"` // this computer holds the node's onion secret
	DiskBytes int64 `json:"disk_bytes"`
	// NodeExit says why the node last stopped (shown while it is not running).
	NodeExit *ExitInfo `json:"node_exit,omitempty"`
	WorkExit *ExitInfo `json:"work_exit,omitempty"`
	LogPath  string    `json:"log_path"`
	Error    string    `json:"error,omitempty"`
}

// Status collects the current state. It never fails: unreachable parts are
// left empty and Reachable/Error say why.
func (m *Manager) Status(ctx context.Context) Status {
	s := m.Settings()
	n, _ := NetworkByID(s.Network)
	st := Status{
		UpdatedAt: time.Now(), Network: s.Network, ChainID: n.ChainID, Initialized: m.Initialized(),
		HasKey: s.Operator != "", Locked: m.Locked(), NiradFound: m.Nirad != "",
		NodeRunning: m.NodeRunning(), WorkerRunning: m.WorkerRunning(), Operator: s.Operator,
	}
	st.LogPath = m.LogPath()
	if e, ok := m.LastExit("node"); ok && !st.NodeRunning {
		st.NodeExit = &e
	}
	if e, ok := m.LastExit("work"); ok && !st.WorkerRunning {
		st.WorkExit = &e
	}
	if !st.Initialized {
		return st
	}
	st.DiskBytes = dirSize(filepath.Join(m.Home(), "data"))
	st.HasOnion = m.HasOnion()
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	ports := m.Ports()
	rpc := ports.RPCURL()
	var cs struct {
		Result struct {
			SyncInfo struct {
				Height     string    `json:"latest_block_height"`
				Time       time.Time `json:"latest_block_time"`
				CatchingUp bool      `json:"catching_up"`
			} `json:"sync_info"`
		} `json:"result"`
	}
	if err := getJSON(ctx, rpc+"/status", &cs); err != nil {
		if st.NodeRunning {
			st.Error = "node is starting"
		}
		return st
	}
	st.Reachable = true
	fmt.Sscan(cs.Result.SyncInfo.Height, &st.Height)
	st.BlockTime, st.CatchingUp = cs.Result.SyncInfo.Time, cs.Result.SyncInfo.CatchingUp
	var ni struct {
		Result struct {
			NPeers string `json:"n_peers"`
		} `json:"result"`
	}
	if getJSON(ctx, rpc+"/net_info", &ni) == nil {
		fmt.Sscan(ni.Result.NPeers, &st.Peers)
	}
	if st.CatchingUp {
		return st // REST answers from a state that is not current yet
	}

	rest := ports.RESTURL()
	var p struct {
		Params struct {
			MinBond string `json:"min_bond"`
		} `json:"params"`
	}
	if getJSON(ctx, rest+"/nira/community/v1/params", &p) == nil {
		st.MinBond = p.Params.MinBond
	}
	var ep struct {
		Epoch struct {
			Number string `json:"number"`
		} `json:"epoch"`
		Next string `json:"next_epoch_height"`
	}
	if getJSON(ctx, rest+"/nira/community/v1/epoch", &ep) == nil {
		st.Epoch, st.NextEpoch = ep.Epoch.Number, ep.Next
	}
	if s.Operator == "" {
		return st
	}
	var bal struct {
		Balance struct {
			Amount string `json:"amount"`
		} `json:"balance"`
	}
	if getJSON(ctx, rest+"/cosmos/bank/v1beta1/balances/"+s.Operator+"/by_denom?denom=anira", &bal) == nil {
		st.Balance = bal.Balance.Amount
	}
	var node struct {
		Node struct {
			Status       string `json:"status"`
			Bond         string `json:"bond"`
			UnitsTotal   string `json:"units_total"`
			RewardsTotal string `json:"rewards_total"`
			SlashedTotal string `json:"slashed_total"`
			UnbondAt     string `json:"unbond_complete_height"`
		} `json:"node"`
	}
	if getJSON(ctx, rest+"/nira/community/v1/nodes/"+s.Operator, &node) == nil {
		switch node.Node.Status {
		case "NODE_STATUS_ACTIVE":
			st.NodeStatus = "ACTIVE"
		case "NODE_STATUS_UNBONDING":
			st.NodeStatus = "UNBONDING"
		}
		st.Bond, st.UnitsTotal, st.UnbondAt = node.Node.Bond, node.Node.UnitsTotal, node.Node.UnbondAt
		st.RewardsTotal, st.SlashedTotal = node.Node.RewardsTotal, node.Node.SlashedTotal
	}
	return st
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
