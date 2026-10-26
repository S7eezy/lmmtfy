package payload

import (
	"bytes"
	"crypto/sha256"
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
)

// Apple Silicon refuses to run a binary whose code signature does not match
// its content. The Go linker signs darwin binaries "ad hoc": no certificate,
// just a code directory holding the SHA-256 of every 4 KiB page of the file.
// Patching the slot changes some pages, so their hashes must be recomputed.
// The signature keeps its size and layout; only hashes change, so this
// works from any OS without Apple tools.
//
// Format reference: Apple's xnu cs_blobs.h, and the Go linker's own
// cmd/internal/codesign which writes these signatures.

const (
	lcCodeSignature       = 0x1d
	csMagicEmbeddedSig    = 0xfade0cc0
	csMagicCodeDirectory  = 0xfade0c02
	csSlotCodeDirectory   = 0
	csAdhoc               = 0x2
	csHashTypeSHA256      = 2
	codeDirectoryMinBytes = 40
)

// codeDirectory locates the page hashes of an ad-hoc signature.
type codeDirectory struct {
	hashes    []byte // nSlots consecutive SHA-256 hashes, inside the binary
	nSlots    int
	codeLimit int
	pageSize  int
}

// Resign recomputes the ad-hoc signature of a Mach-O executable, in place.
// Other files, and unsigned Mach-O files (darwin/amd64 may be), are left
// unchanged.
func Resign(bin []byte) error {
	cd, err := findCodeDirectory(bin)
	if err != nil || cd == nil {
		return err
	}
	for i := range cd.nSlots {
		start := i * cd.pageSize
		end := min(start+cd.pageSize, cd.codeLimit)
		sum := sha256.Sum256(bin[start:end])
		copy(cd.hashes[i*sha256.Size:], sum[:])
	}
	return nil
}

// VerifySignature checks every page hash of an ad-hoc signed Mach-O file. It
// returns nil for files without signature.
func VerifySignature(bin []byte) error {
	cd, err := findCodeDirectory(bin)
	if err != nil || cd == nil {
		return err
	}
	for i := range cd.nSlots {
		start := i * cd.pageSize
		end := min(start+cd.pageSize, cd.codeLimit)
		sum := sha256.Sum256(bin[start:end])
		if !bytes.Equal(cd.hashes[i*sha256.Size:(i+1)*sha256.Size], sum[:]) {
			return fmt.Errorf("code signature: page %d does not match", i)
		}
	}
	return nil
}

// findCodeDirectory returns nil, nil when bin is not a signed Mach-O file.
func findCodeDirectory(bin []byte) (*codeDirectory, error) {
	f, err := macho.NewFile(bytes.NewReader(bin))
	if err != nil {
		return nil, nil // not Mach-O: nothing to sign
	}
	var dataOff, dataSize uint32
	found := false
	for _, l := range f.Loads {
		raw := l.Raw()
		if len(raw) >= 16 && f.ByteOrder.Uint32(raw) == lcCodeSignature {
			dataOff, dataSize = f.ByteOrder.Uint32(raw[8:]), f.ByteOrder.Uint32(raw[12:])
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	if uint64(dataOff)+uint64(dataSize) > uint64(len(bin)) {
		return nil, errors.New("code signature: out of the file")
	}

	// The signature is a "super blob" (big-endian) indexing other blobs.
	be := binary.BigEndian
	sig := bin[dataOff : dataOff+dataSize]
	if len(sig) < 12 || be.Uint32(sig) != csMagicEmbeddedSig {
		return nil, errors.New("code signature: unknown format")
	}
	count := int(be.Uint32(sig[8:]))
	if count != 1 {
		// More than a code directory: requirements or a certificate. That
		// is a real signature, which only its owner can redo.
		return nil, errors.New("code signature is not ad hoc: sign the converter again after building it")
	}
	if len(sig) < 20 || be.Uint32(sig[12:]) != csSlotCodeDirectory {
		return nil, errors.New("code signature: no code directory")
	}
	cdOff := be.Uint32(sig[16:])
	if uint64(cdOff)+codeDirectoryMinBytes > uint64(len(sig)) {
		return nil, errors.New("code signature: truncated code directory")
	}
	cd := sig[cdOff:]
	if be.Uint32(cd) != csMagicCodeDirectory {
		return nil, errors.New("code signature: bad code directory")
	}
	flags := be.Uint32(cd[12:])
	hashOff := be.Uint32(cd[16:])
	nSlots := int(be.Uint32(cd[28:]))
	codeLimit := int(be.Uint32(cd[32:]))
	hashSize, hashType, pageBits := cd[36], cd[37], cd[39]
	switch {
	case flags&csAdhoc == 0:
		return nil, errors.New("code signature is not ad hoc: sign the converter again after building it")
	case hashType != csHashTypeSHA256 || hashSize != sha256.Size:
		return nil, fmt.Errorf("code signature: unsupported hash type %d", hashType)
	case codeLimit > int(dataOff) || pageBits == 0 || pageBits > 16:
		return nil, errors.New("code signature: inconsistent code directory")
	}
	pageSize := 1 << pageBits
	if want := (codeLimit + pageSize - 1) / pageSize; nSlots != want {
		return nil, fmt.Errorf("code signature: %d page hashes for %d pages", nSlots, want)
	}
	if uint64(hashOff)+uint64(nSlots*sha256.Size) > uint64(len(cd)) {
		return nil, errors.New("code signature: hashes out of bounds")
	}
	return &codeDirectory{
		hashes:    cd[hashOff : int(hashOff)+nSlots*sha256.Size],
		nSlots:    nSlots,
		codeLimit: codeLimit,
		pageSize:  pageSize,
	}, nil
}
