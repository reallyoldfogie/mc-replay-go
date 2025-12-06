package mcpr

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

// ValidateFile performs comprehensive validation of an MCPR file.
// It checks zip integrity, required files, and metadata validity.
// This is automatically called by recorder.Close() when writing to a file.
func ValidateFile(path string) error {
	// Check file exists and has size
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("replay file not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("replay file is empty (0 bytes)")
	}

	// Open as zip
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("not a valid zip file: %w", err)
	}
	defer zr.Close()

	// Check for required files
	fileMap := make(map[string]*zip.File)
	for _, f := range zr.File {
		fileMap[f.Name] = f
	}

	// Validate recording.tmcpr
	recFile, hasRecording := fileMap["recording.tmcpr"]
	if !hasRecording {
		return fmt.Errorf("missing required file: recording.tmcpr")
	}
	if recFile.UncompressedSize64 == 0 {
		log.Printf("[mcpr] WARNING: recording.tmcpr is empty")
	}

	// Validate and parse metaData.json
	metaFile, hasMetadata := fileMap["metaData.json"]
	if !hasMetadata {
		return fmt.Errorf("missing required file: metaData.json")
	}

	rc, err := metaFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open metaData.json: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read metaData.json: %w", err)
	}

	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("failed to parse metaData.json: %w", err)
	}

	// Validate critical metadata fields
	if meta.FileFormat != "MCPR" {
		log.Printf("[mcpr] WARNING: unexpected file format: %s", meta.FileFormat)
	}
	if meta.FileFormatVersion < 1 || meta.FileFormatVersion > 15 {
		log.Printf("[mcpr] WARNING: unusual file format version: %d", meta.FileFormatVersion)
	}
	if meta.Protocol == 0 {
		log.Printf("[mcpr] WARNING: protocol version is 0")
	}
	if meta.Duration == 0 {
		log.Printf("[mcpr] WARNING: replay duration is 0 ms (very short)")
	}

	// Check optional but expected files
	if _, ok := fileMap["mods.json"]; !ok {
		log.Printf("[mcpr] WARNING: missing optional file: mods.json")
	}
	if _, ok := fileMap["recording.tmcpr.crc32"]; !ok {
		log.Printf("[mcpr] WARNING: missing cache file: recording.tmcpr.crc32")
	}

	// Log validation success with key info
	log.Printf("[mcpr] Validated %s: %s protocol %d, %d ms, %d bytes",
		path, meta.MCVersion, meta.Protocol, meta.Duration, info.Size())

	return nil
}

// ValidateFileQuiet is like ValidateFile but suppresses all log output.
// Useful for CLI tools that want to control output formatting.
func ValidateFileQuiet(path string) error {
	// Temporarily suppress log output
	oldFlags := log.Flags()
	oldOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer func() {
		log.SetFlags(oldFlags)
		log.SetOutput(oldOutput)
	}()

	return ValidateFile(path)
}
