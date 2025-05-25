package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strconv"
	"sync"
	"time"

	"github.com/kaito-root/raft-from-scratch/pkg/raft"
)

type RaftNode struct {
	rf      *raft.Raft
	peers   []int
	applyCh chan raft.ApplyMsg
}

func NewRaftNode(id int, peers []int) *RaftNode {
	applyCh := make(chan raft.ApplyMsg, 100)
	rf := raft.Make(id, peers, applyCh)
	return &RaftNode{
		rf:      rf,
		peers:   peers,
		applyCh: applyCh,
	}
}

func (n *RaftNode) StartRPCServer(port int) {
	rpc.Register(n.rf)
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatal("Accept error:", err)
			}
			go rpc.ServeConn(conn)
		}
	}()
}

func (n *RaftNode) connectToPeers(ports []int) {
	for i, port := range ports {
		if i == n.rf.GetID() {
			continue // Skip self
		}
		go func(peerID, port int) {
			for {
				client, err := rpc.Dial("tcp", "localhost:"+strconv.Itoa(port))
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				n.rf.Mu.Lock()
				n.rf.Clients[peerID] = client
				n.rf.Mu.Unlock()
				break
			}
		}(i, port)
	}
}

func main() {
	// Create 3 Raft nodes
	ports := []int{8001, 8002, 8003}
	peers := []int{0, 1, 2} // Node IDs
	nodes := make([]*RaftNode, len(peers))

	// Start Raft nodes
	for i, port := range ports {
		nodes[i] = NewRaftNode(i, peers)
		nodes[i].StartRPCServer(port)
		fmt.Printf("Started Raft node %d on port %d\n", i, port)
	}

	// Connect nodes to each other
	for _, node := range nodes {
		node.connectToPeers(ports)
	}

	// Wait for connections to establish
	time.Sleep(1 * time.Second)

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Find the leader
	var leader *RaftNode
	for _, node := range nodes {
		term, isLeader := node.rf.GetState()
		if isLeader {
			leader = node
			fmt.Printf("Node %d is leader in term %d\n", node.rf.GetID(), term)
			break
		}
	}

	if leader == nil {
		log.Fatal("No leader elected")
	}

	// Submit some commands to the leader
	commands := []string{"cmd1", "cmd2", "cmd3"}
	for _, cmd := range commands {
		index, term, isLeader := leader.rf.Start(cmd)
		if !isLeader {
			log.Fatal("Leader lost leadership")
		}
		fmt.Printf("Command %s submitted at index %d in term %d\n", cmd, index, term)
	}

	// Wait for log replication
	time.Sleep(1 * time.Second)

	// Check if all nodes have the same log
	firstLog := nodes[0].rf.GetLog()
	for i, node := range nodes {
		if i == 0 {
			continue
		}
		log := node.rf.GetLog()
		if len(log) != len(firstLog) {
			fmt.Printf("Node %d has different log length: got %d, want %d\n", i, len(log), len(firstLog))
		}
		for j, entry := range log {
			if entry.Term != firstLog[j].Term || entry.Command != firstLog[j].Command {
				fmt.Printf("Node %d has different log entry at index %d\n", i, j)
			}
		}
	}

	// Keep the program running
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
