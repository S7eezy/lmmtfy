// Package payload writes a resolved mapping into the payload slot of a
// runner binary, and reads it back at run time.
//
// The slot is a 1 MiB byte array compiled into every runner (package slot),
// starting with a unique marker. Because it is initialized, it is stored in
// the executable file, so `lmmtfy build` can find the marker and overwrite
// the slot in place without changing the file layout:
//
//	[marker, 22 bytes][2 zero bytes][u32 metadata length][metadata JSON]
//	[u32 mapping length][gzip(mapping YAML)][SHA-256 of the previous fields]
//	[zero padding up to 1 MiB]
//
// Lengths are little-endian. An empty slot has zero lengths. The checksum
// makes a damaged converter refuse to run instead of converting with a
// corrupted configuration.
package payload

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// SlotSize is the size of the payload slot.
const SlotSize = 1 << 20

// The marker is spelled backwards here so that only the slot itself holds
// it forwards: a runner binary must contain exactly one occurrence.
const markerReversed = "1v-TOLS-DAOLYAP-YFTMML"

const headerSize = 24 // marker + 2 zero bytes

// Marker returns the bytes opening the slot.
func Marker() []byte {
	b := []byte(markerReversed)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

// ErrEmpty means the slot holds no mapping: the binary is a template.
var ErrEmpty = errors.New("no mapping embedded")

// Metadata describes an embedded mapping.
type Metadata struct {
	Mapping       string `json:"mapping"`
	MappingSHA256 string `json:"mapping_sha256"`
	EngineVersion string `json:"engine_version"`
	Platform      string `json:"platform"`
	BuiltAt       string `json:"built_at"`
	BuiltBy       string `json:"built_by"`
}

// Patch returns a copy of bin with meta and mapping written into its slot.
// meta.MappingSHA256 is set from mapping.
func Patch(bin []byte, meta Metadata, mapping []byte) ([]byte, error) {
	marker := Marker()
	if n := bytes.Count(bin, marker); n != 1 {
		return nil, fmt.Errorf("not a converter template: payload slot found %d times, want 1", n)
	}
	at := bytes.Index(bin, marker)
	if at+SlotSize > len(bin) {
		return nil, errors.New("not a converter template: payload slot is truncated")
	}

	sum := sha256.Sum256(mapping)
	meta.MappingSHA256 = hex.EncodeToString(sum[:])
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	var zipped bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&zipped, gzip.BestCompression)
	zw.Write(mapping)
	if err := zw.Close(); err != nil {
		return nil, err
	}

	content := binary.LittleEndian.AppendUint32(nil, uint32(len(metaJSON)))
	content = append(content, metaJSON...)
	content = binary.LittleEndian.AppendUint32(content, uint32(zipped.Len()))
	content = append(content, zipped.Bytes()...)
	check := sha256.Sum256(content)
	content = append(content, check[:]...)
	if headerSize+len(content) > SlotSize {
		return nil, fmt.Errorf("mapping too large for a converter: %d bytes compressed, at most %d", len(content), SlotSize-headerSize)
	}

	out := bytes.Clone(bin)
	slot := out[at : at+SlotSize]
	clear(slot[headerSize:]) // the template may be a converter built earlier
	copy(slot[headerSize:], content)
	return out, nil
}

// Decode reads the slot content. It returns ErrEmpty for a template, and an
// error when the content is damaged.
func Decode(slot []byte) (*Metadata, []byte, error) {
	if len(slot) < headerSize+8 || !bytes.HasPrefix(slot, Marker()) {
		return nil, nil, errors.New("payload slot marker not found")
	}
	r := bytes.NewReader(slot[headerSize:])
	next := func() ([]byte, error) {
		var n uint32
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			return nil, err
		}
		if int64(n) > int64(r.Len()) {
			return nil, errors.New("payload length out of bounds")
		}
		b := make([]byte, n)
		_, err := io.ReadFull(r, b)
		return b, err
	}

	metaJSON, err := next()
	if err != nil {
		return nil, nil, err
	}
	if len(metaJSON) == 0 {
		return nil, nil, ErrEmpty
	}
	var meta Metadata
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return nil, nil, fmt.Errorf("metadata: %w", err)
	}
	zipped, err := next()
	if err != nil {
		return nil, nil, err
	}
	end := len(slot) - r.Len() // end of the checksummed fields
	var check [sha256.Size]byte
	if _, err := io.ReadFull(r, check[:]); err != nil {
		return nil, nil, err
	}
	if sha256.Sum256(slot[headerSize:end]) != check {
		return nil, nil, errors.New("checksum mismatch")
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, nil, err
	}
	mapping, err := io.ReadAll(zr)
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(mapping)
	if hex.EncodeToString(sum[:]) != meta.MappingSHA256 {
		return nil, nil, errors.New("mapping checksum mismatch")
	}
	return &meta, mapping, nil
}
