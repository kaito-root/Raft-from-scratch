package main

import (
	"log"
	"os"
	"raft-from-scratch/internal/transport"
	"raft-from-scratch/pkg/raft"
	"time"
)

func main() {
	// Set up logging
	log.SetFlags(log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Println("Starting Raft cluster...")

	peers := map[int]string{
		0: "localhost:8000",
		1: "localhost:8001",
		2: "localhost:8002",
	}

	// Create a slice of peer IDs
	peerIDs := make([]int, 0, len(peers))
	for id := range peers {
		peerIDs = append(peerIDs, id)
	}

	// Start each node
	for id := range peers {
		go startNode(id, peerIDs, peers)
	}
	log.Println("All nodes started. Cluster is running.")
	select {} // block forever
}

func startNode(id int, peerIDs []int, addresses map[int]string) {
	// Create transport layer
	trans := transport.NewTCPTransport(addresses)
	log.Printf("Node %d: Transport layer initialized", id)

	// Create apply channel for committed commands
	applyCh := make(chan raft.ApplyMsg)

	// Create and initialize Raft node
	rf := raft.Make(id, peerIDs, applyCh)
	rf.Transport = trans // Set the transport
	log.Printf("Node %d: Raft node initialized", id)

	// Start the transport server
	trans.StartServer(id, rf)
	log.Printf("Node %d: Transport server started on %s", id, addresses[id])

	// Start processing committed commands
	go func() {
		for msg := range applyCh {
			// Process committed commands here
			if msg.CommandValid {
				log.Printf("Node %d: Applying command at index %d: %v", id, msg.CommandIndex, msg.Command)
				// Apply the command to your state machine
				_ = msg.Command // Replace with actual command processing
			}
		}
	}()

	// Start election timer
	rf.ResetElectionTimer()
	log.Printf("Node %d: Election timer started", id)

	// Start election loop
	go func() {
		for {
			select {
			case <-rf.ElectionTimer.C:
				log.Printf("Node %d: Election timer expired, starting election", id)
				rf.StartElection()
			}
		}
	}()

	// Start state monitoring
	go func() {
		for {
			time.Sleep(time.Second)
			term, isLeader := rf.GetState()
			state := "Follower"
			if isLeader {
				state = "Leader"
			} else if term > 0 && rf.GetCurrentState() == raft.Candidate {
				state = "Candidate"
			}
			log.Printf("Node %d: State=%s, Term=%d, LogLength=%d",
				id, state, term, len(rf.GetLog()))
		}
	}()
}
