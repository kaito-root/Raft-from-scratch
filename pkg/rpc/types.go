package rpc

// RequestVoteArgs represents the arguments for a RequestVote RPC
type RequestVoteArgs struct {
	Term         int // Candidate's term
	CandidateID  int // Candidate requesting vote
	LastLogIndex int // Index of candidate's last log entry
	LastLogTerm  int // Term of candidate's last log entry
}

// RequestVoteReply represents the reply for a RequestVote RPC
type RequestVoteReply struct {
	Term        int  // Current term, for candidate to update itself
	VoteGranted bool // True means candidate received vote
}

// LogEntry represents a single entry in the log
type LogEntry struct {
	Term    int    // Term when entry was received
	Command string // Command to be executed
}

// AppendEntriesArgs represents the arguments for an AppendEntries RPC
type AppendEntriesArgs struct {
	Term         int        // Leader's term
	LeaderID     int        // So follower can redirect clients
	PrevLogIndex int        // Index of log entry immediately preceding new ones
	PrevLogTerm  int        // Term of prevLogIndex entry
	Entries      []LogEntry // Log entries to store (empty for heartbeat)
	LeaderCommit int        // Leader's commitIndex
}

// AppendEntriesReply represents the reply for an AppendEntries RPC
type AppendEntriesReply struct {
	Term          int  // Current term, for leader to update itself
	Success       bool // True if follower contained entry matching prevLogIndex and prevLogTerm
	ConflictIndex int  // Index of the first conflicting entry
	ConflictTerm  int  // Term of the conflicting entry
}
