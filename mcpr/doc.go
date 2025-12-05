// Package mcpr provides a streaming writer for ReplayMod (.mcpr) files.
//
// The writer emits a ZIP file containing at least two entries:
//  - recording.tmcpr: stream of [timeBE:int32][lenBE:int32][varint packetId][packet bytes]
//  - metaData.json: replay metadata written on Close()
//
// Packets can be written incrementally as they are received; the writer does
// not buffer all packets in memory. The duration in metadata is computed
// from the maximum timestamp observed. Metadata is written only on Close().
package mcpr

