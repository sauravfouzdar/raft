package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/sauravfouzdar/raft/pkg/config"
	"github.com/sauravfouzdar/raft/pkg/logger"
	"github.com/sauravfouzdar/raft/pkg/protocol"
	"github.com/sauravfouzdar/raft/pkg/raft"
	"github.com/sauravfouzdar/raft/pkg/transport"
)

// KeyValueStore is a simple key-value store that uses Raft for consensus
type KeyValueStore struct {

	mu sync.RWMutex
	data map[string]string
	raft *raft.Node
	logger *logger.Logger
}

// NewKeyValueStore creates a new KeyValueStore
func NewKeyValueStore(raftNode *raft.Node, logger *logger.Logger) *KeyValueStore {
	return &KeyValueStore{
		data: make(map[string]string),
		raft: raftNode,
		logger: logger,
	}
}

// KVCommand represents a command to modify the key-value store
type KVCommand struct {
	// Op is the operation (set or delete)
	Op string `json:"op"`

	// Key is the key
	Key string `json:"key"`

	// Value is the value (only for set)
	Value string `json:"value,omitempty"`
}

// applyCommand applies a command to the key-value store
func (kv *KeyValueStore) applyCommand(entry protocol.LogEntry) error {
	// Skip no-op entries
	if entry.Type == protocol.NoOpEntry {
		return nil
	}

	// Parse command
	var cmd KVCommand
	if err := json.Unmarshal(entry.Command, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	// Apply command
	kv.mu.Lock()
	defer kv.mu.Unlock()

	switch cmd.Op {
	case "set":
		kv.data[cmd.Key] = cmd.Value
		kv.logger.Info("Set %s = %s", cmd.Key, cmd.Value)
	case "delete":
		delete(kv.data, cmd.Key)
		kv.logger.Info("Deleted %s", cmd.Key)
	default:
		return fmt.Errorf("unknown operation: %s", cmd.Op)
	}

	return nil
}

// get returns the value for a key
func (kv *KeyValueStore) get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	value, ok := kv.data[key]
	return value, ok
}

// set sets a key-value pair
func (kv *KeyValueStore) set(key, value string) error {
	// Create command
	cmd := KVCommand{
		Op:    "set",
		Key:   key,
		Value: value,
	}

	// Marshal command
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}

	// Submit to Raft
	if _, err := kv.raft.Submit(data); err != nil {
		return fmt.Errorf("failed to submit command: %v", err)
	}

	return nil
}

// delete deletes a key
func (kv *KeyValueStore) delete(key string) error {
	// Create command
	cmd := KVCommand{
		Op:  "delete",
		Key: key,
	}

	// Marshal command
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}

	// Submit to Raft
	if _, err := kv.raft.Submit(data); err != nil {
		return fmt.Errorf("failed to submit command: %v", err)
	}

	return nil
}

// handleHTTP sets up HTTP handlers for the key-value store
func (kv *KeyValueStore) handleHTTP() {
	// GET /kv/{key} - Get a value
	http.HandleFunc("/kv/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			key := strings.TrimPrefix(r.URL.Path, "/kv/")
			if key == "" {
				http.Error(w, "Key is required", http.StatusBadRequest)
				return
			}

			value, ok := kv.get(key)
			if !ok {
				http.Error(w, "Key not found", http.StatusNotFound)
				return
			}

			w.Write([]byte(value))
		} else if r.Method == "PUT" {
			// Parse key and value from URL and form
			key := strings.TrimPrefix(r.URL.Path, "/kv/")
			if key == "" {
				http.Error(w, "Key is required", http.StatusBadRequest)
				return
			}

			if err := r.ParseForm(); err != nil {
				http.Error(w, "Failed to parse form", http.StatusBadRequest)
				return
			}

			value := r.Form.Get("value")
			if value == "" {
				http.Error(w, "Value is required", http.StatusBadRequest)
				return
			}

			// Set key-value pair
			if err := kv.set(key, value); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else if r.Method == "DELETE" {
			key := strings.TrimPrefix(r.URL.Path, "/kv/")
			if key == "" {
				http.Error(w, "Key is required", http.StatusBadRequest)
				return
			}

			// Delete key
			if err := kv.delete(key); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// GET /status - Get Raft status
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		state, term := kv.raft.GetState()
		status := struct {
			State     string `json:"state"`
			Term      uint64 `json:"term"`
			IsLeader  bool   `json:"is_leader"`
			KeyCount  int    `json:"key_count"`
		}{
			State:     state.String(),
			Term:      term,
			IsLeader:  kv.raft.IsLeader(),
			KeyCount:  len(kv.data),
		}

		data, err := json.Marshal(status)
		if err != nil {
			http.Error(w, "Failed to marshal status", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
}

func main() {
	// Parse command-line flags
	nodeID := flag.String("id", "node1", "Node ID")
	addr := flag.String("addr", "localhost:8000", "Node address")
	peerList := flag.String("peers", "", "Comma-separated list of peers in the format id=addr,id=addr,...")
	httpAddr := flag.String("http", "localhost:8080", "HTTP server address")
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

	// Create Raft node without transport
	node, err := raft.NewNode(cfg, nil) // We'll set the apply function later
	if err != nil {
		log.Fatalf("Failed to create Raft node: %v", err)
	}

	// Create key-value store
	kv := NewKeyValueStore(node, logger)
	
	// Set the apply function for the node
	node.ApplyFunc = kv.applyCommand

	// Create transport
	trans := transport.NewTCPTransport(*nodeID, *addr, peers, logger)
	
	// Set transport on node
	node.SetTransport(trans)

	// Start Raft node
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start Raft node: %v", err)
	}

	// Set up HTTP server
	kv.handleHTTP()
	go func() {
		logger.Info("Starting HTTP server on %s", *httpAddr)
		if err := http.ListenAndServe(*httpAddr, nil); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	logger.Info("Key-value store started with Raft node ID %s", *nodeID)
	logger.Info("HTTP server listening on %s", *httpAddr)
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