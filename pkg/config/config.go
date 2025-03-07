package config

import (
	"time"
)

// Config holds the configs for a Raft node
type Config struct {
	// NodeID is the unique identifier for this node
	NodeID string

	// Address is the network address of this node (e.g., "localhost:8000")
	Address string

	// Peers is a map of peer node IDs to their network addresses
	Peers map[string]string

	// HeartbeatTimeout is the interval between leader heartbeats
	HeartbeatTimeout time.Duration

	// ElectionTimeoutMin is the minimum time before a follower becomes a candidate
	ElectionTimeoutMin time.Duration

	// ElectionTimeoutMax is the maximum time before a follower becomes a candidate
	ElectionTimeoutMax time.Duration

	// StoragePath is the path where persistent state will be stored
	StoragePath string

	// LogLevel controls the verbosity of logging (debug, info, warn, error)
	LogLevel string
}

// DefaultConfig returns a Config with sensible default values
func DefaultConfig() *Config {
	return &Config{
		NodeID:            "node1",
		Address:           "localhost:8000",
		Peers:             make(map[string]string),
		HeartbeatTimeout:  100 * time.Millisecond,
		ElectionTimeoutMin: 150 * time.Millisecond,
		ElectionTimeoutMax: 300 * time.Millisecond,
		StoragePath:       "./raft-data",
		LogLevel:          "info",
	}
}
