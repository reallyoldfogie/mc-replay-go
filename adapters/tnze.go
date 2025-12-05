// Package tnze provides a tiny adapter to feed packets from github.com/Tnze/go-mc
// (pk.Packet) into an MCPR recorder.
package adapters

import (
	"log"

	pk "github.com/Tnze/go-mc/net/packet"

	"github.com/reallyoldfogie/mc-replay-go/mcpr/recorder"
)

// PacketFunc returns a handler function compatible with mc-agent's packet handler
// signature (func(pk.Packet) error). It records each received clientbound packet
// using the provided recorder.
func PacketFunc(rec *recorder.Recorder) func(pk.Packet) error {
	recordCount := 0
	return func(p pk.Packet) error {
		// Clone payload since upstream may reuse buffers
		data := make([]byte, len(p.Data))
		copy(data, p.Data)
		recordCount++
		if recordCount%100 == 0 {
			log.Printf("[Recorder] Recorded %d packets (latest: ID=%d len=%d)", recordCount, p.ID, len(data))
		}
		return rec.RecordNow(int32(p.ID), data)
	}
}
