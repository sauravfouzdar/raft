# raft
Raft implementation on a key-value pair store


# Raft Consensus Algorithm Implementation in Go



# Getting Started with Raft Implementation

This document guides you through setting up, building, and testing the Raft consensus algorithm implementation.

## Project Structure

```
raft/
├── cmd/
│   └── raftnode/
│       └── main.go              # Entry point to run a Raft node
├── pkg/
│   ├── config/
│   │   └── config.go            # Configuration for Raft nodes
│   ├── logger/
│   │   └── logger.go            # Simple logging utility
│   ├── raft/
│   │   ├── node.go              # Raft node implementation
│   │   ├── state.go             # State management (follower, candidate, leader)
│   │   ├── log.go               # Log entry and management
│   │   ├── storage.go           # Persistent storage interface
│   │   ├── rpc.go               # RPC messages and handlers
│   │   └── timer.go             # Election and heartbeat timers
│   └── transport/
│       └── transport.go         # Network transport layer
├── internal/
│   └── util/
│       └── util.go              # Internal utilities
├── test/
│   ├── integration/
│   │   └── cluster_test.go      # Cluster integration tests
│   └── scripts/
│       └── run_local_cluster.sh # Script to run a local cluster
├── examples/
│   ├── simple_kv/
│   │   └── main.go              # Example key-value store using Raft
│   └── README.md                # Example documentation
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── Makefile                     # Build and test commands
└── README.md                    # Project documentation
```

## Prerequisites

To run this project, you need:

- Go 1.16 or higher
- git
- A macOS environment (macOS M4 MacBook) 

## Project Setup

1. Clone the repository:

```bash
git clone https://github.com/sauravfouzdar/raft.git
cd raft
```

2. Initialize the Go module (if needed):

```bash
go mod tidy
```

## Build and Run

### Building the Project

# To build the Raft node binary:

```bash
make build
```

```bash
make run-kv
```

# Step 3: Wait for leader election Wait a few seconds for the cluster to elect a leader. Check which node is the leader:
```bash
curl http://localhost:8082/status
```

# Step 4: Set a value with leader Node
``` curl -X PUT -d "value=hello_world" http://localhost:8081/kv/mykey ```

# Step 5: Read the value from Node 2 or Node 3 to verify replication
``` curl http://localhost:8082/kv/mykey ```

# Step 6: Delete the value from the leader Node
``` curl -X DELETE http://localhost:8081/kv/mykey ```