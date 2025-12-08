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
//
// PacketFunc returns a handler function that records packets
// with special handling for bundle delimiters. Bundle delimiters are recorded
// with empty payloads to avoid including unconsumed buffer data.
//
// Bundle delimiters only mark the start/end of a bundle - the actual bundled
// packets are recorded individually, so no data is lost.
//
// If bundleDelimiterID is -1, no bundle filtering is applied.
func PacketFunc(rec *recorder.Recorder, bundleDelimiterID int32) func(pk.Packet) error {
	recordCount := 0

	if bundleDelimiterID != -1 {
		log.Printf("[Recorder] Bundle delimiter detection enabled (ID=%d)", bundleDelimiterID)
	}

	return func(p pk.Packet) error {
		var data []byte

		// Bundle delimiters should be recorded with empty payload
		// to avoid recording unconsumed network buffer data
		if bundleDelimiterID != -1 && p.ID == bundleDelimiterID {
			data = []byte{}
			log.Printf("[Recorder] Recording bundle delimiter with empty payload (ID=%d)", p.ID)
		} else {
			// Clone payload since upstream may reuse buffers
			data = make([]byte, len(p.Data))
			copy(data, p.Data)
		}

		recordCount++
		if recordCount%100 == 0 {
			log.Printf("[Recorder] Recorded %d packets (latest: ID=%d len=%d)", recordCount, p.ID, len(data))
		}
		return rec.RecordNow(int32(p.ID), data)
	}
}
