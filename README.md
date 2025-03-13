# raft
Raft implementation on a key-value pair store

## getting started with Raft implementation
This document guides you through setting up, building, and testing the Raft consensus algorithm implementation.

## prerequisites

To run this project, you need:

- Go 1.16 or higher
- git
- A macOS environment (macOS M4 MacBook) 

## project setup

1. Clone the repository:

```bash
git clone https://github.com/sauravfouzdar/raft.git
cd raft
```

2. Initialize the Go module (if needed):

```bash
go mod tidy
```

## build and run

### building the project

# to build the Raft node binary:

```bash
make build
```

```bash
make run-kv
```

## step 3: wait for leader election Wait a few seconds for the cluster to elect a leader. Check which node is the leader:
```bash
curl http://localhost:8082/status
```

## step 4: set a value with leader Node
``` curl -X PUT -d "value=hello_world" http://localhost:8081/kv/mykey ```

## step 5: read the value from Node 2 or Node 3 to verify replication
``` curl http://localhost:8082/kv/mykey ```

## step 6: delete the value from the leader Node
``` curl -X DELETE http://localhost:8081/kv/mykey ```

## Contributing
Pull requests are welcome. Would love to hear your feedback and suggestions.

## References
[Paper](https://raft.github.io/raft.pdf)  
[visualization](https://thesecretlivesofdata.com/raft/)

