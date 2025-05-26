package raft

import (
	"log"
	"math/rand"
	"raft-from-scratch/pkg/rpc"
	"sync/atomic"
	"time"
)

func (rf *Raft) resetElectionTimer() {
	// Reset the election timer to a random duration between 150ms and 300ms
	if rf.ElectionTimer != nil {
		rf.ElectionTimer.Stop()
	}
	rf.ElectionTimer = time.NewTimer(time.Duration(150+rand.Intn(150)) * time.Millisecond)
}

func (rf *Raft) sendRequestVote(server int, args *rpc.RequestVoteArgs, reply *rpc.RequestVoteReply) bool {
	// This function sends a RequestVote RPC to the specified server.
	// It should handle the RPC call and return true if the call was successful.
	log.Printf("Node %d: Sending RequestVote to node %d (term=%d)", rf.id, server, args.Term)
	ok := rf.Transport.SendRequestVote(server, args, reply)
	if ok {
		log.Printf("Node %d: Received RequestVote reply from node %d (term=%d, granted=%v)",
			rf.id, server, reply.Term, reply.VoteGranted)
	} else {
		log.Printf("Node %d: Failed to send RequestVote to node %d", rf.id, server)
	}
	return ok
}

func (rf *Raft) handleRequestVote(args *rpc.RequestVoteArgs) rpc.RequestVoteReply {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply := rpc.RequestVoteReply{
		Term:        rf.currentTerm,
		VoteGranted: false,
	}
	if args.Term < rf.currentTerm {
		// If the term in the request is less than the current term, reject the vote
		log.Printf("Node %d: Rejecting vote request from node %d (term %d < current term %d)",
			rf.id, args.CandidateID, args.Term, rf.currentTerm)
		reply.Term = rf.currentTerm
		return reply
	}
	if args.Term > rf.currentTerm {
		// Update to new term and become follower
		log.Printf("Node %d: Stepping down to follower (new term %d)", rf.id, args.Term)
		rf.currentTerm = args.Term
		rf.votedFor = nil
		rf.state = Follower
	}

	if (rf.votedFor == nil || *rf.votedFor == args.CandidateID) && rf.isLogUptoDate(args) {
		rf.votedFor = &args.CandidateID
		reply.VoteGranted = true
		rf.resetElectionTimer() // Reset the election timer
		log.Printf("Node %d: Granted vote to node %d", rf.id, args.CandidateID)
	} else {
		log.Printf("Node %d: Rejected vote for node %d (already voted or log not up to date)",
			rf.id, args.CandidateID)
	}
	reply.Term = rf.currentTerm
	return reply
}

func (rf *Raft) isLogUptoDate(args *rpc.RequestVoteArgs) bool {
	if len(rf.log) == 0 {
		return true // If our log is empty, we consider it up-to-date
	}
	lastLogIndex := len(rf.log) - 1
	lastLogTerm := rf.log[lastLogIndex].Term
	if args.LastLogTerm > lastLogTerm {
		return true // Candidate's log term is more recent
	} else if args.LastLogTerm == lastLogTerm {
		return args.LastLogIndex >= lastLogIndex // Same term, check index
	}
	return false // Our log is more up-to-date
}

// Todo: Implement the function startElection
// 1. Increment current term
// 2. Vote for self
// 3. Send RequestVote RPCs to all other servers
// 4. Reset election timer

func (rf *Raft) startElection() {
	rf.mu.Lock()

	// Increment current term and become candidate and vote for self
	rf.currentTerm++
	termStarted := rf.currentTerm
	rf.state = Candidate
	rf.votedFor = &rf.id // Vote for self
	log.Printf("Node %d: Starting election for term %d", rf.id, termStarted)

	voteReceived := int32(1) // Count self vote
	totalPeers := len(rf.peers)
	lastLogIndex := len(rf.log) - 1 // Index of last log entry
	lastLogTerm := 0
	if lastLogIndex >= 0 {
		lastLogTerm = rf.log[lastLogIndex].Term // Term of last log entry
	}
	rf.resetElectionTimer()
	rf.mu.Unlock()

	// Send RequestVote RPCs to all other servers
	args := rpc.RequestVoteArgs{
		Term:         termStarted,
		CandidateID:  rf.id,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}
	for _, peer := range rf.peers {
		if peer == rf.id {
			continue
		}
		go func(peerId int) {
			reply := rpc.RequestVoteReply{}
			ok := rf.sendRequestVote(peerId, &args, &reply)
			if !ok {
				return
			}
			rf.mu.Lock()
			defer rf.mu.Unlock()
			if rf.currentTerm != termStarted || rf.state != Candidate {
				return
			}
			if reply.VoteGranted {
				votes := atomic.AddInt32(&voteReceived, 1)
				log.Printf("Node %d: Received vote from node %d (total votes: %d/%d)",
					rf.id, peerId, votes, totalPeers)
				if votes > int32(totalPeers/2) {
					log.Printf("Node %d: Won election for term %d", rf.id, termStarted)
					rf.becomeLeader()
				}
			} else if reply.Term > rf.currentTerm {
				// back to follower since another server has higher term
				log.Printf("Node %d: Stepping down to follower (higher term %d from node %d)",
					rf.id, reply.Term, peerId)
				rf.currentTerm = reply.Term
				rf.state = Follower
				rf.votedFor = nil
				rf.resetElectionTimer()
			}
		}(peer)
	}
}

// A candidate wins an election if it receives votes from a majority of the servers.
//
//	It then becomes the leader.
//	As leader, it sends initial empty AppendEntries (heartbeats) to all followers to assert authority and maintain control.
//
// TODO: Implement the function becomeLeader
// 1. Set state to Leader
// 2. Initialize nextIndex and matchIndex for all followers
// 3. Start heartbeat timer to send AppendEntries RPCs periodically
// 4. Reset election timer
// 5. Send initial empty AppendEntries RPCs to all followers
// 6. Notify apply channel that a new leader has been elected
// 7. Log the election event
// 8. Return
// 9. Handle any errors that may occur during the process
func (rf *Raft) becomeLeader() {
	log.Printf("Node %d: Becoming leader for term %d", rf.id, rf.currentTerm)
	rf.state = Leader
	lastLogIndex := len(rf.log)
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))
	for i := range rf.peers {
		rf.nextIndex[i] = lastLogIndex + 1 // Next index to send to each follower
		rf.matchIndex[i] = 0               // No entries matched yet
	}
	if rf.heartbeatTimer != nil {
		rf.heartbeatTimer.Stop()
	}
	rf.heartbeatTimer = time.NewTicker(100 * time.Millisecond) // Start heartbeat timer
	rf.ElectionTimer.Stop()                                    // Stop the election timer since we are now a leader

	// Start heartbeat loop
	go func() {
		for {
			select {
			case <-rf.heartbeatTimer.C:
				rf.mu.Lock()
				if rf.state != Leader {
					rf.mu.Unlock()
					return
				}
				rf.mu.Unlock()
				log.Printf("Node %d: Sending heartbeats to all followers", rf.id)
				rf.replicateLog() // Send heartbeats to all followers
			}
		}
	}()

	// Send initial empty AppendEntries RPCs to all followers
	log.Printf("Node %d: Sending initial heartbeats to all followers", rf.id)
	rf.replicateLog()
}

func (rf *Raft) replicateLog() {
	rf.mu.Lock()
	if rf.state != Leader {
		rf.mu.Unlock()
		return
	}
	term := rf.currentTerm
	leaderId := rf.id
	leaderCommit := rf.commitIndex
	rf.mu.Unlock()

	for _, peer := range rf.peers {
		if peer == rf.id {
			continue
		}
		go func(peerId int) {
			rf.mu.Lock()
			if rf.state != Leader {
				rf.mu.Unlock()
				return
			}
			nextIndex := rf.nextIndex[peerId]
			prevLogIndex := nextIndex - 1 // Previous log index for heartbeat
			prevLogTerm := 0
			if prevLogIndex >= 0 && prevLogIndex < len(rf.log) {
				prevLogTerm = rf.log[prevLogIndex].Term // Get term of previous log entry
			}
			entries := make([]rpc.LogEntry, len(rf.log[nextIndex:]))
			copy(entries, rf.log[nextIndex:])

			args := rpc.AppendEntriesArgs{
				Term:         term,
				LeaderID:     leaderId,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: leaderCommit,
			}
			rf.mu.Unlock()

			reply := rpc.AppendEntriesReply{}
			ok := rf.sendAppendEntries(peerId, &args, &reply)
			if !ok {
				log.Printf("Node %d: Failed to send heartbeat to node %d", rf.id, peerId)
				return
			}

			rf.mu.Lock()
			defer rf.mu.Unlock()
			if reply.Term > rf.currentTerm {
				log.Printf("Node %d: Stepping down to follower (higher term %d from node %d)",
					rf.id, reply.Term, peerId)
				rf.currentTerm = reply.Term
				rf.state = Follower
				rf.votedFor = nil
				rf.resetElectionTimer()
				return
			}
			if reply.Success {
				// If the heartbeat was successful, update nextIndex and matchIndex
				rf.nextIndex[peerId] = args.PrevLogIndex + len(args.Entries) + 1 // Next index to send
				rf.matchIndex[peerId] = args.PrevLogIndex + len(args.Entries)    // Update match index
				rf.updateCommitIndex()                                           // Update commit index based on match indices
			} else {
				// handle conflict
				if reply.ConflictTerm != -1 {
					conflictIndex := -1
					for i := reply.ConflictIndex; i >= 0; i-- {
						if rf.log[i].Term == reply.ConflictTerm {
							conflictIndex = i
							break
						}
					}
					if conflictIndex >= 0 {
						rf.nextIndex[peerId] = conflictIndex + 1 // Set next index to the first conflicting entry
					} else {
						rf.nextIndex[peerId] = reply.ConflictIndex // If no conflict term found, set to conflict index
					}
				}
			}
		}(peer)
	}
}

func (rf *Raft) sendAppendEntries(server int, args *rpc.AppendEntriesArgs, reply *rpc.AppendEntriesReply) bool {
	// This function sends an AppendEntries RPC to the specified server.
	// It should handle the RPC call and return true if the call was successful.
	if len(args.Entries) == 0 {
		log.Printf("Node %d: Sending heartbeat to node %d (term=%d)", rf.id, server, args.Term)
	} else {
		log.Printf("Node %d: Sending AppendEntries to node %d (term=%d, entries=%d)",
			rf.id, server, args.Term, len(args.Entries))
	}
	ok := rf.Transport.SendAppendEntries(server, args, reply)
	if ok {
		if len(args.Entries) == 0 {
			log.Printf("Node %d: Received heartbeat reply from node %d (term=%d, success=%v)",
				rf.id, server, reply.Term, reply.Success)
		} else {
			log.Printf("Node %d: Received AppendEntries reply from node %d (term=%d, success=%v)",
				rf.id, server, reply.Term, reply.Success)
		}
	} else {
		log.Printf("Node %d: Failed to send AppendEntries to node %d", rf.id, server)
	}
	return ok
}

func (rf *Raft) updateCommitIndex() {
	n := len(rf.log) - 1
	for N := n; N >= 0; N-- {
		count := 1 // Count the leader itself
		for i := range rf.peers {
			if i != rf.id && rf.matchIndex[i] >= N {
				count++ // Count followers that have at least N entries
			}
		}
		if count > len(rf.peers)/2 && rf.log[N].Term == rf.currentTerm {
			rf.commitIndex = N // Update commit index if a majority has this entry
			break
		}
	}
}
