package raft

import (
	"math/rand"
	"sync/atomic"
	"time"
)

func (rf *Raft) resetElectionTimer() {
	// Reset the election timer to a random duration between 150ms and 300ms
	if rf.electionTimer != nil {
		rf.electionTimer.Stop()
	}
	rf.electionTimer = time.NewTimer(time.Duration(150+rand.Intn(150)) * time.Millisecond)
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	// This function sends a RequestVote RPC to the specified server.
	// It should handle the RPC call and return true if the call was successful.
	// The implementation of this function is not shown here.
	ok := callRequestVote(rf.peers, args, reply)
	return ok
}

func (rf *Raft) handleRequestVote(args *RequestVoteArgs) RequestVoteReply {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply := RequestVoteReply{
		Term:        rf.currentTerm,
		VoteGranted: false,
	}
	if args.Term < rf.currentTerm {
		// If the term in the request is less than the current term, reject the vote
		reply.Term = rf.currentTerm
		return reply
	}
	if args.Term > rf.currentTerm {
		// Update to new term and become follower
		rf.currentTerm = args.Term
		rf.votedFor = nil
		rf.state = Follower
	}

	if (rf.votedFor == nil || *rf.votedFor == args.CandidateId) && rf.isLogUptoDate(args) {
		rf.votedFor = &args.CandidateId
		reply.VoteGranted = true
		rf.resetElectionTimer() // Reset the election timer
	}
	reply.Term = rf.currentTerm
	return reply
}

func (rf *Raft) isLogUptoDate(args *RequestVoteArgs) bool {
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
	args := RequestVoteArgs{
		Term:         termStarted,
		CandidateId:  rf.id,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}
	for _, peer := range rf.peers {
		if peer == rf.id {
			continue
		}
		go func(peerId int) {
			reply := RequestVoteReply{}
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
				if atomic.AddInt32(&voteReceived, 1) > int32(totalPeers/2) {
					rf.becomeLeader()
				}
			} else if reply.Term > rf.currentTerm {
				// back to follower since another server has higher term
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
	rf.electionTimer.Stop()                                    // Stop the election timer since we are now a leader
	rf.replicateLog()                                          // Send initial empty AppendEntries RPCs to all followers

	// (Optional) Notify apply channel, log election, etc.
}

func (rf *Raft) replicateLog() {
	rf.mu.Lock()
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
			nextIndex := rf.nextIndex[peerId]
			prevLogIndex := nextIndex - 1 // Previous log index for heartbeat
			prevLogTerm := 0
			if prevLogIndex >= 0 && prevLogIndex < len(rf.log) {
				prevLogTerm = rf.log[prevLogIndex].Term // Get term of previous log entry
			}
			entries := make([]LogEntry, len(rf.log[nextIndex:]))
			copy(entries, rf.log[nextIndex:])

			args := AppendEntriesArgs{
				Term:         term,
				LeaderId:     leaderId,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: leaderCommit,
			}
			rf.mu.Unlock()

			reply := AppendEntriesReply{}
			ok := rf.sendAppendEntries(peerId, &args, &reply)
			if !ok {
				return // Handle error if needed
			}

			rf.mu.Lock()
			defer rf.mu.Unlock()
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.state = Follower
				rf.votedFor = nil
				rf.resetElectionTimer()
				return // Step down if reply term is greater
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

	go func() {
		select {
		case <-rf.heartbeatTimer.C:
			rf.mu.Lock()
			if rf.state != Leader {
				rf.mu.Unlock()
				return
			}
			rf.mu.Unlock()
			rf.replicateLog() // Resend heartbeats if still leader
		}
	}()
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	// This function sends an AppendEntries RPC to the specified server.
	// It should handle the RPC call and return true if the call was successful.
	// The implementation of this function is not shown here.
	ok := callAppendEntries(rf.peers, args, reply)

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
