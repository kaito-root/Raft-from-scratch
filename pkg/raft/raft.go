package raft

import (
	"math/rand"
	"raft-from-scratch/internal/transport"
	"raft-from-scratch/pkg/rpc"
	"sync"
	"time"
)

type State int

const (
	Follower State = iota
	Candidate
	Leader
)

type ApplyMsg struct {
	CommandValid bool        // True if the command is valid
	Command      interface{} // Command to be applied
	CommandIndex int         // Index of the command in the log
}

type Raft struct {
	mu          sync.Mutex // Lock to protect shared access to this peer's state
	peers       []int      // RPC end points of all peers
	id          int        // this peer's index into peers[]
	dead        int32      // set by Kill()
	state       State
	currentTerm int
	votedFor    *int
	log         []rpc.LogEntry

	// Volatile state on all servers
	commitIndex int // Index of highest log entry known to be committed
	lastApplied int // Index of highest log entry applied to state machine

	// Volatile state on leaders
	nextIndex  []int // For each server, index of the next log entry to send
	matchIndex []int // For each server, index of highest log entry known to be replicated

	// Channels for communication
	applyCh chan ApplyMsg

	// timer
	// electionTimeout is the duration after which a follower becomes a candidate
	ElectionTimer *time.Timer
	// heartbeatTimer is used to send periodic heartbeats to followers
	heartbeatTimer *time.Ticker
	Transport      transport.Transport
}

func (rf *Raft) make(id int, peers []int, applyCh chan ApplyMsg) *Raft {
	rf = &Raft{
		id:             id,
		peers:          peers,
		state:          Follower,
		currentTerm:    0,
		votedFor:       nil,
		log:            []rpc.LogEntry{},
		commitIndex:    0,
		lastApplied:    0,
		nextIndex:      make([]int, len(peers)),
		matchIndex:     make([]int, len(peers)),
		applyCh:        applyCh,
		ElectionTimer:  time.NewTimer(randomElectionTimeout()),
		heartbeatTimer: time.NewTicker(100 * time.Millisecond), // Default heartbeat interval
	}
	go rf.applyLogs() // Start applying logs in a separate goroutine
	return rf
}

func (rf *Raft) applyLogs() {
	for {
		rf.mu.Lock()
		for rf.lastApplied < rf.commitIndex {
			rf.lastApplied++
			entry := rf.log[rf.lastApplied-1] // Get the entry to apply

			// Create message while holding the lock
			msg := ApplyMsg{
				CommandValid: true,
				Command:      entry.Command,
				CommandIndex: rf.lastApplied,
			}

			// Release lock before sending to channel to avoid deadlock
			rf.mu.Unlock()
			rf.applyCh <- msg
			rf.mu.Lock()
		}
		rf.mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Sleep to avoid busy waiting
	}
}

func randomElectionTimeout() time.Duration {
	// Randomly choose an election timeout between 150ms and 300ms
	return time.Duration(150+rand.Intn(150)) * time.Millisecond
}

// GetState returns the current term and whether this server believes it is the leader
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == Leader
}

// GetCurrentState returns the current state of the node
func (rf *Raft) GetCurrentState() State {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.state
}

// GetID returns the server's ID
func (rf *Raft) GetID() int {
	return rf.id
}

// GetLog returns a copy of the server's log
func (rf *Raft) GetLog() []rpc.LogEntry {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	log := make([]rpc.LogEntry, len(rf.log))
	copy(log, rf.log)
	return log
}

// Start the agreement process for a new command
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Leader {
		return -1, -1, false
	}

	// Convert command to string
	cmdStr, ok := command.(string)
	if !ok {
		return -1, -1, false
	}

	entry := rpc.LogEntry{
		Term:    rf.currentTerm,
		Command: cmdStr,
	}
	rf.log = append(rf.log, entry)
	return len(rf.log), rf.currentTerm, true
}

// Make creates a new Raft server instance
func Make(id int, peers []int, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	return rf.make(id, peers, applyCh)
}

// HandleRequestVote implements the RPCHandler interface
func (rf *Raft) HandleRequestVote(args *rpc.RequestVoteArgs) (*rpc.RequestVoteReply, error) {
	reply := rf.handleRequestVote(args)
	return &reply, nil
}

// HandleAppendEntries implements the RPCHandler interface
func (rf *Raft) HandleAppendEntries(args *rpc.AppendEntriesArgs) (*rpc.AppendEntriesReply, error) {
	reply := rf.handleAppendEntries(args)
	return &reply, nil
}

// ResetElectionTimer resets the election timer
func (rf *Raft) ResetElectionTimer() {
	if rf.ElectionTimer != nil {
		rf.ElectionTimer.Stop()
	}
	rf.ElectionTimer = time.NewTimer(time.Duration(150+rand.Intn(150)) * time.Millisecond)
}

// StartElection starts a new election
func (rf *Raft) StartElection() {
	rf.startElection()
}
