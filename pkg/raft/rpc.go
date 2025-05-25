package raft

// RPCs for Raft consensus algorithm

// To begin an election, a candidate sends a RequestVote RPCs to each of the other servers.

type RequestVoteArgs struct {
	Term         int // Candidate's term
	CandidateId  int // Candidate requesting vote
	LastLogIndex int // Index of candidate's last log entry
	LastLogTerm  int // Term of candidate's last log entry
}

type RequestVoteReply struct {
	Term        int  // Current term, for candidate to update itself
	VoteGranted bool // True if vote granted
}

//Leaders send periodic heartbeats (AppendEntries RPCs with no entries) to all followers to maintain authority

type AppendEntriesArgs struct {
	Term         int        // Leader's term
	LeaderId     int        // Leader's ID
	PrevLogIndex int        // Index of log entry immediately preceding new ones
	PrevLogTerm  int        // Term of PrevLogIndex entry
	Entries      []LogEntry // Log entries to store (empty for heartbeat)
	LeaderCommit int        // Leader's commit index
}

type AppendEntriesReply struct {
	Term    int  // Current term, for leader to update itself
	Success bool // True if follower contained entry matching PrevLogIndex and PrevLogTerm
	// If false, contains the index of the first log entry that does not match
	ConflictIndex int // Index of the first log entry that does not match
	ConflictTerm  int // Term of the first log entry that does not match
}
