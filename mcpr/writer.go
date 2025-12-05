package mcpr

import (
    "archive/zip"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "hash"
    "hash/crc32"
    "io"
    "os"
    "time"
)

// Writer streams packets into a ReplayMod .mcpr file.
//
// Usage:
//  w, _ := mcpr.Create("out.mcpr", mcpr.Meta{Protocol: 754})
//  defer w.Close()
//  _ = w.WritePacket(0, 0x26, payload)
//
// Packets are written incrementally; the writer does not retain them in memory.
type Writer struct {
    zw       *zip.Writer
    recw     io.Writer
    meta     Meta
    duration uint32
    closed   bool
    file     *os.File  // optional, when using Create()
    crc32    hash.Hash32 // CRC32 hash for recording.tmcpr validation
}

// NewWriter creates a new MCPR writer onto the provided io.Writer.
// It immediately creates the first ZIP entry "recording.tmcpr" and expects
// packets to be written there until Close() is called.
func NewWriter(out io.Writer, meta Meta) (*Writer, error) {
    zw := zip.NewWriter(out)
    rec, err := zw.Create("recording.tmcpr")
    if err != nil {
        return nil, fmt.Errorf("create recording.tmcpr: %w", err)
    }

    if meta.FileFormat == "" {
        meta.FileFormat = "MCPR"
    }
    if meta.FileFormatVersion == 0 {
        meta.FileFormatVersion = CurrentFileFormatVersion
    }
    if meta.Date == 0 {
        meta.Date = time.Now().UnixMilli()
    }

    // Initialize CRC32 hash for cache validation
    crc := crc32.NewIEEE()

    return &Writer{
        zw:    zw,
        recw:  io.MultiWriter(rec, crc), // Write to both file and CRC
        meta:  meta,
        crc32: crc,
    }, nil
}

// Create opens/creates a file at path and returns a Writer that owns the file descriptor.
// Close() will also close the underlying file.
func Create(path string, meta Meta) (*Writer, error) {
    f, err := os.Create(path)
    if err != nil {
        return nil, err
    }
    w, err := NewWriter(f, meta)
    if err != nil {
        _ = f.Close()
        return nil, err
    }
    w.file = f
    return w, nil
}

// WritePacket writes a single packet frame to recording.tmcpr.
// ts is a millisecond timestamp. packetID is the protocol packet id and
// payload the raw packet bytes as they would appear on the wire after the varint id.
func (w *Writer) WritePacket(ts uint32, packetID int32, payload []byte) error {
    if w.closed || w.recw == nil {
        return fmt.Errorf("mcpr: writer closed")
    }

    // Header: time (int32 BE), length (int32 BE) of [varint id + payload]
    var hdr [8]byte
    binary.BigEndian.PutUint32(hdr[0:4], ts)
    varid := encodeVarInt(packetID)
    total := uint32(len(varid) + len(payload))
    binary.BigEndian.PutUint32(hdr[4:8], total)

    if _, err := w.recw.Write(hdr[:]); err != nil {
        return err
    }
    if _, err := w.recw.Write(varid); err != nil {
        return err
    }
    if _, err := w.recw.Write(payload); err != nil {
        return err
    }

    if ts > w.duration {
        w.duration = ts
    }
    return nil
}

// SetSelfID updates the selfId field written to metaData.json.
// ReplayMod uses this to identify the recorder's own player entity.
func (w *Writer) SetSelfID(id int) {
    w.meta.SelfID = id
}

// AddPlayer adds a player UUID to the replay metadata.
// This populates the "players" array in metaData.json for ReplayMod compatibility.
func (w *Writer) AddPlayer(uuid string) {
    // Ensure players array exists
    if w.meta.Players == nil {
        w.meta.Players = []string{}
    }
    // Check for duplicates
    for _, p := range w.meta.Players {
        if p == uuid {
            return
        }
    }
    w.meta.Players = append(w.meta.Players, uuid)
}

// CreateEntry creates a new ZIP entry for additional files (e.g., assets).
// Note: ZIP requires sequential entry writing. Only call this after you have
// finished writing packets; you cannot resume writing to recording.tmcpr afterward.
func (w *Writer) CreateEntry(name string) (io.Writer, error) {
    if w.closed {
        return nil, fmt.Errorf("mcpr: writer closed")
    }
    return w.zw.Create(name)
}

// Close finalizes the recording, writes metaData.json, and closes the archive.
func (w *Writer) Close() error {
    if w.closed {
        return nil
    }
    // Write metaData.json as the last entry
    w.meta.Duration = int(w.duration)
    if w.meta.Generator == "" {
        w.meta.Generator = "mc-replay-go"
    }
    if w.meta.FileFormat == "" {
        w.meta.FileFormat = "MCPR"
    }
    if w.meta.FileFormatVersion == 0 {
        w.meta.FileFormatVersion = CurrentFileFormatVersion
    }

    md, err := w.zw.Create("metaData.json")
    if err != nil {
        return fmt.Errorf("create metaData.json: %w", err)
    }
    b, err := json.Marshal(w.meta)
    if err != nil {
        return fmt.Errorf("marshal metaData.json: %w", err)
    }
    if _, err := md.Write(b); err != nil {
        return err
    }

    // Write mods.json for compatibility with ReplayMod
    modsJSON := map[string][]interface{}{
        "requiredMods": {},
    }
    modsEntry, err := w.zw.Create("mods.json")
    if err != nil {
        return fmt.Errorf("create mods.json: %w", err)
    }
    modsBytes, err := json.Marshal(modsJSON)
    if err != nil {
        return fmt.Errorf("marshal mods.json: %w", err)
    }
    if _, err := modsEntry.Write(modsBytes); err != nil {
        return err
    }

    // Write recording.tmcpr.crc32 for cache validation
    crc32Entry, err := w.zw.Create("recording.tmcpr.crc32")
    if err != nil {
        return fmt.Errorf("create recording.tmcpr.crc32: %w", err)
    }
    crc32Value := fmt.Sprintf("%d", w.crc32.Sum32())
    if _, err := crc32Entry.Write([]byte(crc32Value)); err != nil {
        return err
    }

    if err := w.zw.Close(); err != nil {
        return err
    }
    w.closed = true
    if w.file != nil {
        return w.file.Close()
    }
    return nil
}
