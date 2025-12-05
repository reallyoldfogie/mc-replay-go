package main

import (
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "strconv"
    "strings"

    "github.com/reallyoldfogie/mc-replay-go/mcpr"
)

type packetSpec struct {
    ts   uint32
    id   int32
    data []byte
}

type packetFlags []packetSpec

func (p *packetFlags) String() string { return fmt.Sprintf("%d packets", len(*p)) }

// Format: ts:id:hexpayload  e.g., 1500:38:0AFFEE
func (p *packetFlags) Set(v string) error {
    parts := strings.Split(v, ":")
    if len(parts) != 3 {
        return fmt.Errorf("invalid --packet, want ts:id:hexpayload")
    }
    ts64, err := parseUint(parts[0])
    if err != nil {
        return fmt.Errorf("ts: %w", err)
    }
    id64, err := parseInt(parts[1])
    if err != nil {
        return fmt.Errorf("id: %w", err)
    }
    payload, err := hex.DecodeString(parts[2])
    if err != nil {
        return fmt.Errorf("hexpayload: %w", err)
    }
    *p = append(*p, packetSpec{ts: uint32(ts64), id: int32(id64), data: payload})
    return nil
}

func parseUint(s string) (uint64, error) {
    if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
        return strconv.ParseUint(s[2:], 16, 64)
    }
    return strconv.ParseUint(s, 10, 64)
}

func parseInt(s string) (int64, error) {
    if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
        v, err := strconv.ParseUint(s[2:], 16, 32)
        return int64(v), err
    }
    return strconv.ParseInt(s, 10, 32)
}

func main() {
    var out string
    var protocol int
    var generator string
    var pkts packetFlags

    flag.StringVar(&out, "out", "example.mcpr", "Output .mcpr path")
    flag.IntVar(&protocol, "protocol", 754, "MC network protocol (e.g. 754 for 1.16.5)")
    flag.StringVar(&generator, "generator", "mc-replay-go", "Generator string in metadata")
    flag.Var(&pkts, "packet", "Packet spec ts:id:hexpayload (repeatable)")
    flag.Parse()

    w, err := mcpr.Create(out, mcpr.Meta{Protocol: protocol, Generator: generator})
    if err != nil {
        log.Fatalf("create writer: %v", err)
    }
    defer func() {
        if err := w.Close(); err != nil {
            log.Fatalf("close: %v", err)
        }
    }()

    // If no packets provided, still produce a valid empty replay
    for _, sp := range pkts {
        if err := w.WritePacket(sp.ts, sp.id, sp.data); err != nil {
            log.Fatalf("write packet: %v", err)
        }
    }

    fmt.Printf("wrote %s (%d packets)\n", out, len(pkts))
}

