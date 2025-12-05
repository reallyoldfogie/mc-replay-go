package main

import (
    "bufio"
    "bytes"
    "compress/zlib"
    "context"
    "errors"
    "flag"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "sync"
    "time"

    "github.com/reallyoldfogie/mc-replay-go/mcpr"
)

// Minimal TCP proxy that records server->client Minecraft packets into an MCPR file.
//
// Limitations:
// - Only handles a single client connection and exits after it closes.
// - Compression support is optional and limited: it can auto-detect SetCompression (login id=0x03) for many versions.
// - Does not attempt protocol translation; it simply splits frames and records packet id + payload.

func main() {
    var listen, upstream, out string
    var protocol int
    var generator string
    var assumeNoCompress bool
    var guessCompress bool
    var forceThreshold int

    flag.StringVar(&listen, "listen", ":25566", "Local listen address (proxy)")
    flag.StringVar(&upstream, "upstream", "127.0.0.1:25565", "Upstream Minecraft server address")
    flag.StringVar(&out, "out", "proxy.mcpr", "Output .mcpr path")
    flag.IntVar(&protocol, "protocol", 754, "MC network protocol number (e.g. 754)")
    flag.StringVar(&generator, "generator", "mc-replay-go/proxyrec", "Generator string for metadata")
    flag.BoolVar(&assumeNoCompress, "no-compress", false, "Assume server never enables compression")
    flag.BoolVar(&guessCompress, "guess-compress", true, "Detect login SetCompression and enable compression handling")
    flag.IntVar(&forceThreshold, "compression-threshold", -1, "Force compression enabled with given threshold (>=0)")
    flag.Parse()

    ln, err := net.Listen("tcp", listen)
    if err != nil {
        log.Fatalf("listen: %v", err)
    }
    log.Printf("listening on %s, proxying to %s", listen, upstream)

    conn, err := ln.Accept()
    if err != nil {
        log.Fatalf("accept: %v", err)
    }
    defer conn.Close()
    _ = ln.Close()

    upstreamConn, err := net.Dial("tcp", upstream)
    if err != nil {
        log.Fatalf("dial upstream: %v", err)
    }
    defer upstreamConn.Close()

    w, err := mcpr.Create(out, mcpr.Meta{Protocol: protocol, Generator: generator, ServerName: upstream})
    if err != nil {
        log.Fatalf("create writer: %v", err)
    }
    // Graceful shutdown on SIGINT/SIGTERM
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    start := time.Now()

    var wg sync.WaitGroup
    // Client->Server (just proxy)
    wg.Add(1)
    go func() {
        defer wg.Done()
        go func() { <-ctx.Done(); _ = conn.Close() }()
        io.Copy(upstreamConn, conn)
        _ = upstreamConn.(*net.TCPConn).CloseWrite()
    }()

    // Server->Client (proxy + record via tee)
    wg.Add(1)
    go func() {
        defer wg.Done()
        defer func() { _ = conn.(*net.TCPConn).CloseWrite() }()
        go func() { <-ctx.Done(); _ = upstreamConn.Close() }()
        // Create a pipe feeding the parser without slowing down forwarding
        pr, pw := io.Pipe()
        var parseWG sync.WaitGroup
        parseWG.Add(1)
        go func() {
            defer parseWG.Done()
            if err := parseAndRecord(pr, w, start, assumeNoCompress, guessCompress, forceThreshold); err != nil && !errors.Is(err, io.EOF) {
                log.Printf("parser: %v", err)
            }
        }()

        // Forward raw bytes and tee into parser
        if err := forwardWithTee(upstreamConn, conn, pw); err != nil && err != io.EOF {
            log.Printf("forward: %v", err)
        }
        _ = pw.Close()
        parseWG.Wait()
    }()

    wg.Wait()

    if err := w.Close(); err != nil {
        log.Printf("close writer: %v", err)
    } else {
        log.Printf("finalized %s", out)
    }
}

// forwardWithTee copies from src to dst and mirrors the bytes into tee.
// It stops on any read/write error and returns it; returns io.EOF on clean close.
func forwardWithTee(src io.Reader, dst io.Writer, tee io.Writer) error {
    buf := make([]byte, 32*1024)
    for {
        n, rerr := src.Read(buf)
        if n > 0 {
            if _, werr := dst.Write(buf[:n]); werr != nil {
                return werr
            }
            if tee != nil {
                if _, terr := tee.Write(buf[:n]); terr != nil {
                    // end parser
                    _ = terr
                }
            }
        }
        if rerr != nil {
            return rerr
        }
    }
}

// parseAndRecord reads framed packets from r and writes them to the replay writer.
// It assumes MC VarInt length framing, optional zlib compression (threshold unknown, inferred),
// and stops once framing becomes invalid (e.g., encryption starts) or EOF.
func parseAndRecord(r io.Reader, w *mcpr.Writer, start time.Time, assumeNoCompress, guessCompress bool, forceThreshold int) error {
    br := bufio.NewReader(r)
    compressionEnabled := false
    if forceThreshold >= 0 {
        compressionEnabled = true
    }
    const maxFrame = 8 << 20 // 8 MiB safety cap
    for {
        // Each packet frame: VarInt length, then 'length' bytes of data
        frameLen, err := readVarInt(br)
        if err != nil {
            return err
        }
        if frameLen <= 0 || frameLen > maxFrame {
            return fmt.Errorf("invalid frame length %d", frameLen)
        }
        frame := make([]byte, frameLen)
        if _, err := io.ReadFull(br, frame); err != nil {
            return err
        }

        // Decode for recording
        data := frame
        if !assumeNoCompress && compressionEnabled {
            // When compression is enabled, the first VarInt is uncompressed size
            zr := bytes.NewReader(data)
            uncompressedSize, err := readVarInt(zr)
            if err == nil {
                if uncompressedSize > 0 {
                    // Decompress the rest of zr
                    z, err := zlib.NewReader(zr)
                    if err == nil {
                        var buf bytes.Buffer
                        if _, err := io.Copy(&buf, z); err == nil {
                            data = buf.Bytes()
                        }
                        _ = z.Close()
                    }
                    // If decompressor failed, fall back to raw payload below
                } else {
                    // Data was not compressed, remainder is payload
                    data, _ = io.ReadAll(zr)
                }
            }
        }

        // Now data should start with VarInt packetID followed by payload
        pr := bytes.NewReader(data)
        pid64, err := readVarInt(pr)
        if err != nil {
            // framing invalid beyond this point; stop recording
            return err
        }
        pid := int32(pid64)
        payload, _ := io.ReadAll(pr)

        // Heuristic: detect SetCompression during login if requested
        if guessCompress && !compressionEnabled && !assumeNoCompress {
            // If payload is exactly one VarInt and nothing else, assume SetCompression
            if _, ok := singleVarInt(payload); ok {
                compressionEnabled = true
            }
        }

        ts := uint32(time.Since(start).Milliseconds())
        if err := w.WritePacket(ts, pid, payload); err != nil {
            return err
        }
    }
}

// readVarInt reads a VarInt as used in the MC protocol and returns it as int64.
func readVarInt(r io.ByteReader) (int64, error) {
    var num int64
    var shift uint
    for i := 0; i < 5; i++ {
        b, err := r.ReadByte()
        if err != nil {
            return 0, err
        }
        num |= int64(b&0x7F) << shift
        if (b & 0x80) == 0 {
            return num, nil
        }
        shift += 7
    }
    return 0, fmt.Errorf("varint too long")
}

func writeVarInt(w io.Writer, v int) error {
    var buf [5]byte
    i := 0
    uv := uint32(v)
    for {
        b := byte(uv & 0x7F)
        uv >>= 7
        if uv != 0 {
            b |= 0x80
        }
        buf[i] = b
        i++
        if uv == 0 {
            break
        }
    }
    _, err := w.Write(buf[:i])
    return err
}

// singleVarInt returns the value if the payload decodes to exactly one VarInt and EOF.
func singleVarInt(b []byte) (int64, bool) {
    r := bytes.NewReader(b)
    val, err := readVarInt(r)
    if err != nil {
        return 0, false
    }
    if r.Len() == 0 {
        return val, true
    }
    return 0, false
}
