package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kaito-root/raft-from-scratch/pkg/raft"
)

// Simulate a network of Raft nodes
type Network struct {
	mu      sync.Mutex
	rafts   map[int]*raft.Raft
	applyCh map[int]chan raft.ApplyMsg
}

func NewNetwork() *Network {
	return &Network{
		rafts:   make(map[int]*raft.Raft),
		applyCh: make(map[int]chan raft.ApplyMsg),
	}
}

func (n *Network) AddNode(id int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Create a channel for this node
	n.applyCh[id] = make(chan raft.ApplyMsg, 100)

	// Get all peer IDs
	peers := make([]int, 0, len(n.rafts))
	for peerID := range n.rafts {
		peers = append(peers, peerID)
	}
	peers = append(peers, id)

	// Create the Raft instance
	rf := raft.Make(id, peers, n.applyCh[id])
	n.rafts[id] = rf
}

func (n *Network) RemoveNode(id int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.rafts, id)
	close(n.applyCh[id])
	delete(n.applyCh, id)
}

func TestRaftElection(t *testing.T) {
	network := NewNetwork()

	// Create 5 nodes
	for i := 0; i < 5; i++ {
		network.AddNode(i)
	}

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Check if a leader was elected
	leaderCount := 0
	for _, rf := range network.rafts {
		term, isLeader := rf.GetState()
		if isLeader {
			leaderCount++
			fmt.Printf("Node %d is leader in term %d\n", rf.GetID(), term)
		}
	}

	if leaderCount != 1 {
		t.Errorf("Expected 1 leader, got %d", leaderCount)
	}

	// Simulate network partition
	fmt.Println("\nSimulating network partition...")
	network.RemoveNode(0)
	network.RemoveNode(1)

	// Wait for new leader election
	time.Sleep(2 * time.Second)

	// Check if a new leader was elected
	leaderCount = 0
	for _, rf := range network.rafts {
		term, isLeader := rf.GetState()
		if isLeader {
			leaderCount++
			fmt.Printf("Node %d is leader in term %d\n", rf.GetID(), term)
		}
	}

	if leaderCount != 1 {
		t.Errorf("Expected 1 leader after partition, got %d", leaderCount)
	}
}

func TestRaftLogReplication(t *testing.T) {
	network := NewNetwork()

	// Create 5 nodes
	for i := 0; i < 5; i++ {
		network.AddNode(i)
	}

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Find the leader
	var leader *raft.Raft
	for _, rf := range network.rafts {
		_, isLeader := rf.GetState()
		if isLeader {
			leader = rf
			break
		}
	}

	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Submit some commands to the leader
	commands := []string{"cmd1", "cmd2", "cmd3"}
	for _, cmd := range commands {
		index, term, isLeader := leader.Start(cmd)
		if !isLeader {
			t.Fatal("Leader lost leadership")
		}
		fmt.Printf("Command %s submitted at index %d in term %d\n", cmd, index, term)
	}

	// Wait for log replication
	time.Sleep(1 * time.Second)

	// Check if all nodes have the same log
	firstLog := network.rafts[0].GetLog()
	for id, rf := range network.rafts {
		if id == 0 {
			continue
		}
		log := rf.GetLog()
		if len(log) != len(firstLog) {
			t.Errorf("Node %d has different log length: got %d, want %d", id, len(log), len(firstLog))
		}
		for i, entry := range log {
			if entry.Term != firstLog[i].Term || entry.Command != firstLog[i].Command {
				t.Errorf("Node %d has different log entry at index %d", id, i)
			}
		}
	}
}
