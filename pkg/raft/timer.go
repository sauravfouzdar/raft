package raft

import (
	"math/rand"
	"sync"
	"time"
)

// Timer manages the election and heartbeat timers for a Raft node
type Timer struct {
	// electionTimeoutMin is the minimum time before a follower becomes a candidate
	electionTimeoutMin time.Duration

	// electionTimeoutMax is the maximum time before a follower becomes a candidate
	electionTimeoutMax time.Duration

	// heartbeatTimeout is the interval between leader heartbeats
	heartbeatTimeout time.Duration

	// electionTimer is the timer for election timeout
	electionTimer *time.Timer

	// heartbeatTimer is the timer for heartbeat timeout
	heartbeatTimer *time.Timer

	// electionTimeoutC is the channel for election timeout events
	electionTimeoutC chan struct{}

	// heartbeatTimeoutC is the channel for heartbeat timeout events
	heartbeatTimeoutC chan struct{}

	// mu protects access to the timers
	mu sync.Mutex

	// random is used to generate random election timeouts
	random *rand.Rand

	// stopped indicates if the timer has been stopped
	stopped bool
}

// NewTimer creates a new Timer
func NewTimer(electionTimeoutMin, electionTimeoutMax, heartbeatTimeout time.Duration) *Timer {
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	t := &Timer{
		electionTimeoutMin: electionTimeoutMin,
		electionTimeoutMax: electionTimeoutMax,
		heartbeatTimeout:   heartbeatTimeout,
		electionTimeoutC:   make(chan struct{}),
		heartbeatTimeoutC:  make(chan struct{}),
		random:             r,
		stopped:            false,
	}

	t.resetElectionTimer()
	t.resetHeartbeatTimer()

	return t
}

// resetElectionTimer resets the election timer with a random timeout
func (t *Timer) resetElectionTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	// Cancel existing timer if it exists
	if t.electionTimer != nil {
		t.electionTimer.Stop()
	}

	// Generate random timeout between min and max
	ms := t.electionTimeoutMin.Milliseconds() +
		t.random.Int63n(t.electionTimeoutMax.Milliseconds()-t.electionTimeoutMin.Milliseconds())
	timeout := time.Duration(ms) * time.Millisecond

	// Create new timer
	t.electionTimer = time.AfterFunc(timeout, func() {
		if !t.stopped {
			t.electionTimeoutC <- struct{}{}
		}
	})
}

// resetHeartbeatTimer resets the heartbeat timer
func (t *Timer) resetHeartbeatTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	// Cancel existing timer if it exists
	if t.heartbeatTimer != nil {
		t.heartbeatTimer.Stop()
	}

	// Create new timer
	t.heartbeatTimer = time.AfterFunc(t.heartbeatTimeout, func() {
		if !t.stopped {
			t.heartbeatTimeoutC <- struct{}{}
		}
	})
}

// ResetElection resets the election timer (called when receiving a heartbeat)
func (t *Timer) ResetElection() {
	t.resetElectionTimer()
}

// ResetHeartbeat resets the heartbeat timer (called when sending a heartbeat)
func (t *Timer) ResetHeartbeat() {
	t.resetHeartbeatTimer()
}

// ElectionTimeout returns a channel that receives a signal when the election times out
func (t *Timer) ElectionTimeout() <-chan struct{} {
	return t.electionTimeoutC
}

// HeartbeatTimeout returns a channel that receives a signal when the heartbeat times out
func (t *Timer) HeartbeatTimeout() <-chan struct{} {
	return t.heartbeatTimeoutC
}

// Stop stops all timers
func (t *Timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopped = true

	if t.electionTimer != nil {
		t.electionTimer.Stop()
	}

	if t.heartbeatTimer != nil {
		t.heartbeatTimer.Stop()
	}
}