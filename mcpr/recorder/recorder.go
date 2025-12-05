// Package recorder provides a simple, thread-safe helper to feed decoded
// Minecraft packets into an MCPR writer. It is transport-agnostic: you call
// RecordNow/RecordAt for each server->client packet you receive from your
// client/bot library.
package recorder

import (
	"sync"
	"time"

	"github.com/reallyoldfogie/mc-replay-go/mcpr"
)

// Recorder streams packets to an underlying mcpr.Writer and computes
// timestamps relative to its start time.
type Recorder struct {
	w      *mcpr.Writer
	start  time.Time
	mu     sync.Mutex
	closed bool
}

// New creates a Recorder writing to the given io.Writer using the provided metadata.
// See mcpr.NewWriter for details. The recorder start time is set to now.
func New(w *mcpr.Writer) *Recorder {
	return &Recorder{w: w, start: time.Now()}
}

// NewFile creates and owns an MCPR file at path using the given metadata.
// Use Close() when finished.
func NewFile(path string, meta mcpr.Meta) (*Recorder, error) {
	w, err := mcpr.Create(path, meta)
	if err != nil {
		return nil, err
	}
	return &Recorder{w: w, start: time.Now()}, nil
}

// RecordNow records a packet with the current timestamp relative to start.
// id is the protocol packet id; payload are the packet bytes after the varint id.
func (r *Recorder) RecordNow(id int32, payload []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	ts := uint32(time.Since(r.start).Milliseconds())
	return r.w.WritePacket(ts, id, payload)
}

// RecordAt records a packet with an explicit millisecond timestamp.
func (r *Recorder) RecordAt(ts uint32, id int32, payload []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	return r.w.WritePacket(ts, id, payload)
}

// Close finalizes the MCPR file (writing metaData.json and ZIP central directory).
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	return r.w.Close()
}

// SetSelfID annotates the recording with the recorder's player entity id.
// Safe to call any time before Close(); no-op after Close().
func (r *Recorder) SetSelfID(id int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.w.SetSelfID(id)
}

// AddPlayer adds a player UUID to the recorded session.
// Safe to call any time before Close(); no-op after Close().
func (r *Recorder) AddPlayer(uuid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.w.AddPlayer(uuid)
}
