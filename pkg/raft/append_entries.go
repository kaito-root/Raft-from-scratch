package raft

func (rf *Raft) handleAppendEntries(args *AppendEntriesArgs) AppendEntriesReply {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply := AppendEntriesReply{
		Term:    rf.currentTerm,
		Success: false,
	}

	if args.Term < rf.currentTerm {
		// Reply false if term is outdated
		return reply
	}

	// if term is newer, step down
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = nil // Reset votedFor when stepping down
		rf.resetElectionTimer()
	}
	// check if log is contains the entry at PrevLogIndex with PrevLogTerm
	if args.PrevLogIndex >= len(rf.log) {
		reply.Success = false
		reply.ConflictIndex = len(rf.log) // Conflict at the end of the log
		reply.ConflictTerm = -1           // No term available for conflict
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
		return reply
	}
	// Append new entries to the log
	rf.log = rf.log[:args.PrevLogIndex+1] // Truncate log to PrevLogIndex
	rf.log = append(rf.log, args.Entries...)
	reply.Success = true
	// Update commit index if necessary
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, len(rf.log)-1)
		// TODO: Apply committed entries to state machine
	}
	return reply

}
