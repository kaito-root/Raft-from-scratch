package raft

import (
	"log"
	"raft-from-scratch/pkg/rpc"
)

func (rf *Raft) handleAppendEntries(args *rpc.AppendEntriesArgs) rpc.AppendEntriesReply {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply := rpc.AppendEntriesReply{
		Term:    rf.currentTerm,
		Success: false,
	}

	if args.Term < rf.currentTerm {
		// Reply false if term is outdated
		log.Printf("Node %d: Rejecting AppendEntries from node %d (term %d < current term %d)",
			rf.id, args.LeaderID, args.Term, rf.currentTerm)
		return reply
	}

	// if term is newer, step down
	if args.Term > rf.currentTerm {
		log.Printf("Node %d: Stepping down to follower (new term %d from leader %d)",
			rf.id, args.Term, args.LeaderID)
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = nil // Reset votedFor when stepping down
		rf.resetElectionTimer()
	}

	// Log heartbeat messages
	if len(args.Entries) == 0 {
		log.Printf("Node %d: Received heartbeat from leader %d (term=%d)",
			rf.id, args.LeaderID, args.Term)
		reply.Success = true
		rf.resetElectionTimer() // Reset election timer on heartbeat
		return reply
	}

	// check if log is contains the entry at PrevLogIndex with PrevLogTerm
	if args.PrevLogIndex >= len(rf.log) {
		reply.Success = false
		reply.ConflictIndex = len(rf.log) // Conflict at the end of the log
		reply.ConflictTerm = -1           // No term available for conflict
		log.Printf("Node %d: Rejecting AppendEntries from node %d (log too short, need index %d)",
			rf.id, args.LeaderID, args.PrevLogIndex)
		return reply
	}
	if args.PrevLogIndex >= 0 && rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		reply.ConflictTerm = rf.log[args.PrevLogIndex].Term

		// Scan backwards to find the first index with the different term
		conflictIndex := args.PrevLogIndex
		for conflictIndex > 0 && rf.log[conflictIndex-1].Term == reply.ConflictTerm {
			conflictIndex--
		}
		reply.ConflictIndex = conflictIndex
		log.Printf("Node %d: Rejecting AppendEntries from node %d (term mismatch at index %d)",
			rf.id, args.LeaderID, args.PrevLogIndex)
		return reply
	}
	// Append new entries to the log
	rf.log = rf.log[:args.PrevLogIndex+1] // Truncate log to PrevLogIndex
	rf.log = append(rf.log, args.Entries...)
	reply.Success = true

	if len(args.Entries) > 0 {
		log.Printf("Node %d: Appended %d entries from leader %d",
			rf.id, len(args.Entries), args.LeaderID)
	}

	// Update commit index if necessary
	if args.LeaderCommit > rf.commitIndex {
		oldCommitIndex := rf.commitIndex
		rf.commitIndex = min(args.LeaderCommit, len(rf.log)-1)
		if rf.commitIndex > oldCommitIndex {
			log.Printf("Node %d: Updated commit index from %d to %d",
				rf.id, oldCommitIndex, rf.commitIndex)
		}
	}
	return reply
}
