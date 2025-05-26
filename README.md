# Raft Implementation from Scratch

This repository contains a complete implementation of the Raft consensus algorithm from scratch in Go. Raft is a consensus algorithm designed to be easy to understand and implement, making it suitable for building distributed systems.

## Overview

The implementation includes:

- Leader election
- Log replication
- Heartbeat mechanism
- State management
- TCP-based RPC communication

## Project Structure

```
.
├── internal/
│   └── transport/         # Network transport layer
│       └── tcp_transport.go
├── pkg/
│   ├── raft/             # Core Raft implementation
│   │   ├── append_entries.go
│   │   ├── election.go
│   │   └── raft.go
│   └── rpc/              # RPC types and interfaces
│       └── types.go
└── main.go               # Example usage and cluster setup
```

## Features

- **Leader Election**: Implements the Raft leader election protocol with randomized election timeouts
- **Log Replication**: Handles log replication from leader to followers
- **Heartbeat Mechanism**: Regular heartbeats to maintain leadership and detect failures
- **State Management**: Proper handling of follower, candidate, and leader states
- **TCP Transport**: Reliable RPC communication using TCP

## Getting Started

### Prerequisites

- Go 1.16 or later

### Running the Example

To run a 3-node Raft cluster:

```bash
go run main.go
```

This will start three nodes on localhost ports 8000, 8001, and 8002.

## Implementation Details

### Leader Election

The implementation follows the Raft paper's leader election protocol:

- Random election timeouts (150-300ms)
- RequestVote RPCs for election
- Majority-based voting
- Term-based state transitions

### Log Replication

Log replication is handled through AppendEntries RPCs:

- Heartbeats for maintaining leadership
- Log consistency checks
- Conflict resolution
- Commit index updates

### State Management

Each node maintains:

- Current term
- Voted for information
- Log entries
- Commit index
- State (Follower/Candidate/Leader)

## Testing

The implementation includes logging for monitoring:

- State changes
- Election events
- Heartbeat messages
- RPC communication

## Contributing

Feel free to submit issues and enhancement requests!

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- The Raft paper: "In Search of an Understandable Consensus Algorithm" by Diego Ongaro and John Ousterhout
  - [Extended Raft Paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)
- MIT 6.824 Distributed Systems Course
  - [Lab 3: Raft](https://pdos.csail.mit.edu/6.824/labs/lab-raft1.html)
- The Go programming language community
