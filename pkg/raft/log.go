package raft

import (
	"encoding/json"
	"fmt"
	"sync"
)

// LogEntryType defines the type of a log entry
type LogEntryType int

const (
	// CommandEntry is a log entry that contains a command for the state machine
	CommandEntry LogEntryType = iota
	// ConfigurationEntry is a log entry that contains a configuration change
	ConfigurationEntry
	// NoOpEntry is a log entry that contains no operation
	NoOpEntry
)

// LogEntry represents an entry in the Raft log
type LogEntry struct {
	// Index is the position of the entry in the log
	Index uint64

	// Term is the term when the entry was received by the leader
	Term uint64

	// Type is the type of the log entry
	Type LogEntryType

	// Command is the command to be applied to the state machine
	Command []byte
}

// String returns a string representation of the LogEntry
func (e LogEntry) String() string {
	return fmt.Sprintf("{Index: %d, Term: %d, Type: %d, Command: %s}", 
		e.Index, e.Term, e.Type, string(e.Command))
}

// Log manages the log entries for a Raft node
type Log struct {
	// entries is the list of log entries
	entries []LogEntry

	// startIndex is the index of the first entry in the log
	// This is used when log compaction is implemented
	startIndex uint64

	// mu protects access to the log
	mu sync.RWMutex
}

// NewLog creates a new Log with an initial empty entry
func NewLog() *Log {
	return &Log{
		entries:    []LogEntry{{Index: 0, Term: 0, Type: NoOpEntry}},
		startIndex: 0,
	}
}

// Append adds a new log entry to the log
func (l *Log) Append(term uint64, entryType LogEntryType, command []byte) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	index := l.getLastIndex() + 1
	entry := LogEntry{
		Index:   index,
		Term:    term,
		Type:    entryType,
		Command: command,
	}

	l.entries = append(l.entries, entry)
	return index, nil
}

// GetLastEntry returns the last entry in the log
func (l *Log) GetLastEntry() LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.entries) == 0 {
		return LogEntry{Index: 0, Term: 0, Type: NoOpEntry}
	}
	return l.entries[len(l.entries)-1]
}

// GetLastIndex returns the index of the last entry in the log
func (l *Log) getLastIndex() uint64 {
	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[len(l.entries)-1].Index
}

// GetLastTerm returns the term of the last entry in the log
func (l *Log) GetLastTerm() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[len(l.entries)-1].Term
}

// GetEntry returns the log entry at the specified index
func (l *Log) GetEntry(index uint64) (LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if index < l.startIndex {
		return LogEntry{}, fmt.Errorf("index %d is less than log start index %d", index, l.startIndex)
	}

	if index-l.startIndex >= uint64(len(l.entries)) {
		return LogEntry{}, fmt.Errorf("index %d is out of bounds (len: %d, startIndex: %d)", 
			index, len(l.entries), l.startIndex)
	}

	return l.entries[index-l.startIndex], nil
}

// GetEntries returns a slice of log entries starting at the specified index
func (l *Log) GetEntries(startIndex uint64, maxEntries int) ([]LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if startIndex < l.startIndex {
		return nil, fmt.Errorf("startIndex %d is less than log start index %d", startIndex, l.startIndex)
	}

	if startIndex-l.startIndex >= uint64(len(l.entries)) {
		return []LogEntry{}, nil
	}

	// Calculate the end index
	endIndex := startIndex - l.startIndex + uint64(maxEntries)
	if endIndex > uint64(len(l.entries)) {
		endIndex = uint64(len(l.entries))
	}

	// Return a copy of the entries
	result := make([]LogEntry, endIndex-(startIndex-l.startIndex))
	copy(result, l.entries[startIndex-l.startIndex:endIndex])
	return result, nil
}

// DeleteEntriesFrom deletes log entries starting at the specified index
func (l *Log) DeleteEntriesFrom(index uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index < l.startIndex {
		return fmt.Errorf("index %d is less than log start index %d", index, l.startIndex)
	}

	if index-l.startIndex >= uint64(len(l.entries)) {
		return nil // No entries to delete
	}

	l.entries = l.entries[:index-l.startIndex]
	return nil
}

// AppendEntries appends entries from another log, starting at the specified index
func (l *Log) AppendEntries(index uint64, entries []LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index < l.startIndex {
		return fmt.Errorf("index %d is less than log start index %d", index, l.startIndex)
	}

	// If the index is beyond the current log end, we need to fill the gap
	if index-l.startIndex > uint64(len(l.entries)) {
		return fmt.Errorf("index %d is beyond the current log end %d", 
			index, l.startIndex+uint64(len(l.entries))-1)
	}

	// Delete conflicting entries
	l.entries = l.entries[:index-l.startIndex]

	// Append new entries
	l.entries = append(l.entries, entries...)
	return nil
}

// GetLogSize returns the number of entries in the log
func (l *Log) GetLogSize() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// SerializeEntry serializes a log entry to JSON
func SerializeEntry(entry LogEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// DeserializeEntry deserializes a log entry from JSON
func DeserializeEntry(data []byte) (LogEntry, error) {
	var entry LogEntry
	err := json.Unmarshal(data, &entry)
	return entry, err
}