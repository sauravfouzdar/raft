# raft
Raft implementation on a key-value pair


# Raft Consensus Algorithm Implementation in Go

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

# Getting Started with Raft Implementation

This document guides you through setting up, building, and testing our Raft consensus algorithm implementation.

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

To build the Raft node binary:

```bash
make build
```

This will create a binary at `bin/raftnode`.

### Running a Single Node

To run a single Raft node:

```bash
make run-node
```

Or manually:

```bash
go run ./cmd/raftnode/main.go --id node1 --addr localhost:8000
```

### Running a Cluster

To run a local cluster of 3 nodes:

```bash
make run-cluster
```

This will start three nodes with the following configuration:
- Node 1: localhost:8001 (HTTP on 8081)
- Node 2: localhost:8002 (HTTP on 8082)
- Node 3: localhost:8003 (HTTP on 8083)

You can also run the cluster manually using the test script:

```bash
chmod +x ./test/scripts/run_local_cluster.sh
./test/scripts/run_local_cluster.sh
```

## Running Tests

### Unit Tests

To run unit tests for all packages:

```bash
make test
```

Or for a specific package:

```bash
go test -v ./pkg/raft
```

### Integration Tests

Integration tests require more time as they start an actual cluster:

```bash
go test -v ./test/integration
```

To skip integration tests when running all tests:

```bash
go test -short ./...
```

## Key-Value Store Example

The project includes a simple key-value store example that uses Raft for consensus.

### Running the Key-Value Store

```bash
go run ./examples/simple_kv/main.go --id node1 --addr localhost:8001 --http localhost:8081
```

### Using the Key-Value Store API

Once the key-value store is running, you can interact with it using HTTP:

- **Get a value**:
  ```bash
  curl http://localhost:8081/kv/mykey
  ```

- **Set a value**:
  ```bash
  curl -X PUT -d 'value=myvalue' http://localhost:8081/kv/mykey
  ```

- **Delete a value**:
  ```bash
  curl -X DELETE http://localhost:8081/kv/mykey
  ```

- **Get node status**:
  ```bash
  curl http://localhost:8081/status
  ```

## Debugging

### Logging

The Raft implementation includes a logger with different verbosity levels:
- debug: Detailed debugging information
- info: General information (default)
- warn: Warning messages
- error: Error messages

To change the log level, use the `--log-level` flag:

```bash
go run ./cmd/raftnode/main.go --id node1 --log-level debug
```

### Data Storage

Raft state is persisted to the `./raft-data` directory (configurable with `--storage`). Each node has its own subdirectory with:
- `state.json`: Current term and voted for
- `log.bin`: Log entries

To reset the state, simply delete the directory:

```bash
rm -rf ./raft-data
```

## Common Problems and Solutions

### Can't Connect to Peers
If nodes can't connect to peers, ensure:
1. All nodes are running
2. Peer addresses are correctly specified
3. No firewalls are blocking communication
4. The `--peers` parameter includes all other nodes

### No Leader Elected
If no leader is elected, check:
1. Node logs for timeout values
2. Network connectivity between nodes
3. Any errors in the logs

### Commands Not Replicated
If commands are not replicated to followers:
1. Ensure you're sending commands to the leader
2. Check leader logs for errors
3. Verify the command format is correct

## Next Steps

After getting familiar with the basic implementation, consider:

1. Implementing client request redirection to the leader
2. Adding log compaction for better performance
3. Implementing cluster membership changes
4. Building a more robust state machine on top of Raft

## License

This project is licensed under the MIT License - see the LICENSE file for details.