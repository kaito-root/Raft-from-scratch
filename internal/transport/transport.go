package transport

import (
	"raft-from-scratch/pkg/rpc"
)

type Transport interface {
	SendRequestVote(serverID int, args *rpc.RequestVoteArgs, reply *rpc.RequestVoteReply) bool
	SendAppendEntries(serverID int, args *rpc.AppendEntriesArgs, reply *rpc.AppendEntriesReply) bool
	StartServer(id int, handler RPCHandler)
	Close()
}

type RPCHandler interface {
	HandleRequestVote(args *rpc.RequestVoteArgs) (*rpc.RequestVoteReply, error)
	HandleAppendEntries(args *rpc.AppendEntriesArgs) (*rpc.AppendEntriesReply, error)
}
