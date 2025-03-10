package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	//"time"

	"github.com/sauravfouzdar/raft/pkg/config"
	"github.com/sauravfouzdar/raft/pkg/logger"
	"github.com/sauravfouzdar/raft/pkg/protocol"
	"github.com/sauravfouzdar/raft/pkg/raft"
	"github.com/sauravfouzdar/raft/pkg/transport"
)

func main() {
	// Parse command-line flags
	nodeID := flag.String("id", "node1", "Node ID")
	addr := flag.String("addr", "localhost:8000", "Node address")
	peerList := flag.String("peers", "", "Comma-separated list of peers in the format id=addr,id=addr,...")
	storageDir := flag.String("storage", "./raft-data", "Storage directory")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Parse peers
	peers := make(map[string]string)
	if *peerList != "" {
		peerPairs := strings.Split(*peerList, ",")
		for _, pair := range peerPairs {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				log.Fatalf("Invalid peer format: %s", pair)
			}
			peers[kv[0]] = kv[1]
		}
	}

	// Add self to peers
	peers[*nodeID] = *addr

	// Create config
	cfg := config.DefaultConfig()
	cfg.NodeID = *nodeID
	cfg.Address = *addr
	cfg.Peers = peers
	cfg.StoragePath = fmt.Sprintf("%s/%s", *storageDir, *nodeID)
	cfg.LogLevel = *logLevel

	// Create logger
	logger := logger.New(*nodeID, *logLevel)

	// Create and start Raft node
	node, err := raft.NewNode(cfg, handleCommand)
	if err != nil {
		log.Fatalf("Failed to create Raft node: %v", err)
	}

	// Create transport
	trans := transport.NewTCPTransport(*nodeID, *addr, peers, logger)
	
	// Set transport on node
	node.SetTransport(trans)

	// Start node
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start Raft node: %v", err)
	}

	logger.Info("Raft node started with ID %s, listening on %s", *nodeID, *addr)
	logger.Info("Peers: %v", peers)

	// Wait for interrupt signal
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	// Shutdown gracefully
	fmt.Println("Shutting down...")
	if err := node.Stop(); err != nil {
		log.Fatalf("Failed to stop Raft node: %v", err)
	}
}

// handleCommand applies a committed log entry to the state machine
func handleCommand(entry protocol.LogEntry) error {
	// For the example, just print the command
	fmt.Printf("Applying command: %s\n", string(entry.Command))
	return nil
}
