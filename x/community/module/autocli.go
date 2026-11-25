package community

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"nira-chain/x/community/types"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
// Bytes arguments accept hex or base64. SubmitTaskProof is driven by the
// `prove` / `run` custom commands (client/cli), which build real ICS-23 proofs.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "Params", Use: "params", Short: "Show the community module parameters"},
				{RpcMethod: "Node", Use: "node [operator]", Short: "Show a community node",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "operator"}}},
				{RpcMethod: "Nodes", Use: "nodes", Short: "List community nodes"},
				{RpcMethod: "CurrentEpoch", Use: "epoch", Short: "Show the current community epoch"},
				{RpcMethod: "Tasks", Use: "tasks [operator]", Short: "Show the operator's derived tasks for the current epoch",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "operator"}}},
				{RpcMethod: "Alert", Use: "alert [id]", Short: "Show a divergence alert",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "id"}}},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "UpdateParams", Skip: true},    // governance only
				{RpcMethod: "ResolveAlert", Skip: true},    // governance only
				{RpcMethod: "SubmitTaskProof", Skip: true}, // use `prove` / `run`
				{RpcMethod: "RegisterNode", Use: "register-node [moniker] [bond] [onion-tip-hex] [onion-length]",
					Short: "Register a community node (tip from `onion-init`)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "moniker"}, {ProtoField: "bond"}, {ProtoField: "onion_tip"}, {ProtoField: "onion_length"}}},
				{RpcMethod: "RotateOnion", Use: "rotate-onion [onion-tip-hex] [onion-length]",
					Short:          "Replace the onion commitment (valid from next epoch)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "onion_tip"}, {ProtoField: "onion_length"}}},
				{RpcMethod: "DeregisterNode", Use: "deregister-node", Short: "Deregister and start bond unbonding"},
				{RpcMethod: "ReportDivergence", Use: "report-divergence [height] [claimed-app-hash-hex]",
					Short:          "Report a locally computed app hash that differs from the committed one (bonded, advisory)",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "height"}, {ProtoField: "claimed_app_hash"}}},
			},
		},
	}
}
