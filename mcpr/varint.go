package mcpr

// encodeVarInt encodes a 32-bit integer as a Minecraft-style VarInt.
// It returns a slice backed by a new allocation of up to 5 bytes.
func encodeVarInt(v int32) []byte {
    uv := uint32(v)
    out := make([]byte, 0, 5)
    for {
        b := byte(uv & 0x7F)
        uv >>= 7
        if uv != 0 {
            b |= 0x80
        }
        out = append(out, b)
        if uv == 0 {
            break
        }
    }
    return out
}

