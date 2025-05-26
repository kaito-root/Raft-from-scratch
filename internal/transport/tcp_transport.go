package transport

import (
	"encoding/gob"
	"log"
	"net"
	"raft-from-scratch/pkg/rpc"
)

type tcpTransport struct {
	addresses map[int]string
	listener  net.Listener
	handler   RPCHandler
}

func NewTCPTransport(addresses map[int]string) Transport {
	return &tcpTransport{addresses: addresses}
}

func (t *tcpTransport) StartServer(id int, handler RPCHandler) {
	t.handler = handler
	ln, err := net.Listen("tcp", t.addresses[id])
	if err != nil {
		panic(err)
	}
	t.listener = ln

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go t.handleConnection(conn)
		}
	}()
}

func (t *tcpTransport) handleConnection(conn net.Conn) {
	defer conn.Close()
	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)

	var rpcType string
	if err := dec.Decode(&rpcType); err != nil {
		log.Printf("Failed to decode RPC type: %v", err)
		return
	}

	switch rpcType {
	case "RequestVote":
		var args rpc.RequestVoteArgs
		if err := dec.Decode(&args); err != nil {
			log.Printf("Failed to decode RequestVote args: %v", err)
			return
		}
		reply, err := t.handler.HandleRequestVote(&args)
		if err != nil {
			log.Printf("Failed to handle RequestVote: %v", err)
			return
		}
		if err := enc.Encode(reply); err != nil {
			log.Printf("Failed to encode RequestVote reply: %v", err)
			return
		}
	case "AppendEntries":
		var args rpc.AppendEntriesArgs
		if err := dec.Decode(&args); err != nil {
			log.Printf("Failed to decode AppendEntries args: %v", err)
			return
		}
		reply, err := t.handler.HandleAppendEntries(&args)
		if err != nil {
			log.Printf("Failed to handle AppendEntries: %v", err)
			return
		}
		if err := enc.Encode(reply); err != nil {
			log.Printf("Failed to encode AppendEntries reply: %v", err)
			return
		}
	default:
		log.Printf("Unknown RPC type: %s", rpcType)
	}
}

func (t *tcpTransport) sendRPC(serverID int, rpcType string, args interface{}, reply interface{}) bool {
	log.Printf("Sending %s RPC to node %d", rpcType, serverID)
	conn, err := net.Dial("tcp", t.addresses[serverID])
	if err != nil {
		log.Printf("Failed to connect to node %d: %v", serverID, err)
		return false
	}
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	if err := enc.Encode(rpcType); err != nil {
		log.Printf("Failed to encode RPC type to node %d: %v", serverID, err)
		return false
	}
	if err := enc.Encode(args); err != nil {
		log.Printf("Failed to encode args to node %d: %v", serverID, err)
		return false
	}
	if err := dec.Decode(reply); err != nil {
		log.Printf("Failed to decode reply from node %d: %v", serverID, err)
		return false
	}
	log.Printf("Successfully completed %s RPC to node %d", rpcType, serverID)
	return true
}

func (t *tcpTransport) SendRequestVote(serverID int, args *rpc.RequestVoteArgs, reply *rpc.RequestVoteReply) bool {
	return t.sendRPC(serverID, "RequestVote", args, reply)
}

func (t *tcpTransport) SendAppendEntries(serverID int, args *rpc.AppendEntriesArgs, reply *rpc.AppendEntriesReply) bool {
	log.Printf("Sending AppendEntries RPC to node %d", serverID)
	return t.sendRPC(serverID, "AppendEntries", args, reply)
}

func (t *tcpTransport) Close() {
	t.listener.Close()
}
