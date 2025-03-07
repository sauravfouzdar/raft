package raft

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage represents the persistent storage for a Raft node
type Storage interface {
	// SaveState saves the current term and voted for
	SaveState(currentTerm uint64, votedFor string) error

	// LoadState loads the current term and voted for
	LoadState() (uint64, string, error)

	// SaveLogEntry saves a log entry to storage
	SaveLogEntry(entry LogEntry) error

	// LoadLogEntries loads all log entries from storage
	LoadLogEntries() ([]LogEntry, error)

	// DeleteLogEntriesFrom deletes log entries starting from the specified index
	DeleteLogEntriesFrom(index uint64) error

	// Close closes the storage
	Close() error
}

// FileStorage implements the Storage interface using files
type FileStorage struct {
	// path is the directory where the data will be stored
	path string

	// stateFile is the path to the file storing the current term and voted for
	stateFile string

	// logFile is the path to the file storing the log entries
	logFile string

	// mu protects access to the storage
	mu sync.Mutex
}

// DeleteLogEntriesFrom deletes log entries starting from the specified index
func (fs *FileStorage) DeleteLogEntriesFrom(index uint64) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Load all entries
	entries, err := fs.LoadLogEntries()
	if err != nil {
		return fmt.Errorf("failed to load log entries: %v", err)
	}

	// Find entries to keep
	var keepEntries []LogEntry
	for _, entry := range entries {
		if entry.Index < index {
			keepEntries = append(keepEntries, entry)
		}
	}

	// Truncate file
	if err := os.Truncate(fs.logFile, 0); err != nil {
		return fmt.Errorf("failed to truncate log file: %v", err)
	}

	// Rewrite file with kept entries
	for _, entry := range keepEntries {
		if err := fs.SaveLogEntry(entry); err != nil {
			return fmt.Errorf("failed to save log entry: %v", err)
		}
	}

	return nil
}

// Close closes the storage
func (fs *FileStorage) Close() error {
	return nil
}

// NewFileStorage creates a new FileStorage
func NewFileStorage(path string) (*FileStorage, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %v", err)
	}

	return &FileStorage{
		path:      path,
		stateFile: filepath.Join(path, "state.json"),
		logFile:   filepath.Join(path, "log.bin"),
	}, nil
}

// SaveState saves the current term and voted for
func (fs *FileStorage) SaveState(currentTerm uint64, votedFor string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	state := struct {
		CurrentTerm uint64 `json:"current_term"`
		VotedFor    string `json:"voted_for"`
	}{
		CurrentTerm: currentTerm,
		VotedFor:    votedFor,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	if err := os.WriteFile(fs.stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	return nil
}

// LoadState loads the current term and voted for
func (fs *FileStorage) LoadState() (uint64, string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// If the file doesn't exist, return default values
	if _, err := os.Stat(fs.stateFile); os.IsNotExist(err) {
		return 0, "", nil
	}

	data, err := os.ReadFile(fs.stateFile)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read state file: %v", err)
	}

	var state struct {
		CurrentTerm uint64 `json:"current_term"`
		VotedFor    string `json:"voted_for"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return 0, "", fmt.Errorf("failed to unmarshal state: %v", err)
	}

	return state.CurrentTerm, state.VotedFor, nil
}

// SaveLogEntry saves a log entry to storage
func (fs *FileStorage) SaveLogEntry(entry LogEntry) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Open file in append mode
	file, err := os.OpenFile(fs.logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	// Serialize the entry
	data, err := SerializeEntry(entry)
	if err != nil {
		return fmt.Errorf("failed to serialize log entry: %v", err)
	}

	// Write entry size as uint32
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(data)))
	if _, err := file.Write(sizeBytes); err != nil {
		return fmt.Errorf("failed to write entry size: %v", err)
	}

	// Write entry data
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write entry data: %v", err)
	}

	return nil
}

// LoadLogEntries loads all log entries from storage
func (fs *FileStorage) LoadLogEntries() ([]LogEntry, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// If the file doesn't exist, return empty slice
	if _, err := os.Stat(fs.logFile); os.IsNotExist(err) {
		return []LogEntry{}, nil
	}

	file, err := os.Open(fs.logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	var entries []LogEntry
	sizeBytes := make([]byte, 4)

	for {
		// Read entry size
		n, err := file.Read(sizeBytes)
		if err != nil || n != 4 {
			if err.Error() == "EOF" {
				break // End of file
			}
			return nil, fmt.Errorf("failed to read entry size: %v", err)
		}

		// Parse entry size
		size := binary.BigEndian.Uint32(sizeBytes)
		
		// Read entry data
		data := make([]byte, size)
		n, err = file.Read(data)
		if err != nil || uint32(n) != size {
			return nil, fmt.Errorf("failed to read entry data: %v", err)
		}

		// Deserialize entry
		entry, err := DeserializeEntry(data)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize entry: %v", err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}