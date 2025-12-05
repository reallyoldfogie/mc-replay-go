mc-replay-go
============

Streaming writer for ReplayMod (.mcpr) files in Go.

Install
-------

  go get github.com/reallyoldfogie/mc-replay-go/mcpr

Usage
-----

Write packets incrementally to a new .mcpr and finalize metadata on Close():

  package main

  import (
    "log"
    "github.com/reallyoldfogie/mc-replay-go/mcpr"
  )

  func main() {
    w, err := mcpr.Create("example.mcpr", mcpr.Meta{
      Protocol: 754, // set to the MC network protocol of your packets
      Generator: "mc-replay-go v0.1.0",
    })
    if err != nil { log.Fatal(err) }
    defer w.Close()

    // Example packet: ts=1500ms, id=0x26, payload=... (wire bytes after varint id)
    // _ = w.WritePacket(1500, 0x26, payload)
  }

Notes
-----
- Packets are streamed; they are not buffered in memory.
- The writer computes duration from the maximum timestamp it sees.
- Metadata (metaData.json) is written at Close().
- To include extra files, call CreateEntry(name) after finishing packets.

Integrate With Your Client/Bot
------------------------------

Use the `recorder` helper to feed decoded server→client packets into a file. You provide the packet id and payload (the bytes after the VarInt id) from your client library.

  import (
    "github.com/reallyoldfogie/mc-replay-go/mcpr"
    "github.com/reallyoldfogie/mc-replay-go/mcpr/recorder"
  )

  // Create once when your connection is established
  rec, err := recorder.NewFile("session.mcpr", mcpr.Meta{Protocol: 770, Generator: "mc-agent"})
  if err != nil { /* handle */ }
  defer rec.Close()

  // In your packet receive loop (server→client only):
  // id: int32, payload: []byte (packet bytes after the VarInt id)
  _ = rec.RecordNow(id, payload)

mc-agent (github.com/reallyoldfogie/mc-agent)
---------------------------------------------

mc-agent uses github.com/Tnze/go-mc and dispatches pk.Packet values to handlers. You can attach the adapter like this:

  import (
    pk "github.com/Tnze/go-mc/net/packet"
    "github.com/reallyoldfogie/mc-replay-go/mcpr"
    "github.com/reallyoldfogie/mc-replay-go/mcpr/recorder"
    tnzeadapter "github.com/reallyoldfogie/mc-replay-go/adapters/tnze"
  )

  // After connecting and knowing the protocol version:
  rec, err := recorder.NewFile("session.mcpr", mcpr.Meta{Protocol: int(protocolVersion), Generator: "mc-agent"})
  if err != nil { /* handle */ }
  // Ensure Close() on shutdown
  defer rec.Close()

  // Register a generic handler for clientbound packets
  a.Events().AddGeneric(agent.PacketHandler{Priority: 0, F: tnzeadapter.PacketFunc(rec)})

This records every clientbound packet after decode, which avoids transport-layer encryption and compression concerns.

CLI
---

Build and run the example creator:

  go run ./cmd/mcpr-create -out example.mcpr -protocol 754 \
    --packet 0:0x26:0AFFEE \
    --packet 1500:0x3A:DEADBEEF

Each --packet is ts:id:hexpayload. If you omit --packet, it creates a valid empty replay.

Integration Example: Proxy Recorder
-----------------------------------

Record a live session by proxying a client through to a server and capturing server→client packets:

  go run ./examples/proxyrec -listen :25566 -upstream 127.0.0.1:25565 \
    -out proxy.mcpr -protocol 754

Then connect your Minecraft client to localhost:25566. When you disconnect, the proxy closes and writes proxy.mcpr.

Notes:
- Handles one client connection. Intended for testing.
- Compression is heuristically supported; use -no-compress if your server disables compression.
- The recorder does not parse packet contents; it splits network frames and records id+payload.
