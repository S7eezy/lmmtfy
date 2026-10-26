package payload

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
)

// fakeBinary looks like a runner: bytes around an empty slot.
func fakeBinary() []byte {
	var b bytes.Buffer
	b.WriteString(strings.Repeat("code", 1000))
	slot := make([]byte, SlotSize)
	copy(slot, Marker())
	b.Write(slot)
	b.WriteString(strings.Repeat("more", 1000))
	return b.Bytes()
}

func slotOf(t *testing.T, bin []byte) []byte {
	t.Helper()
	at := bytes.Index(bin, Marker())
	return bin[at : at+SlotSize]
}

func TestPatchDecodeRoundTrip(t *testing.T) {
	bin := fakeBinary()
	if _, _, err := Decode(slotOf(t, bin)); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty slot: err = %v, want ErrEmpty", err)
	}

	mapping := []byte("version: 1\nname: acme\n")
	out, err := Patch(bin, Metadata{Mapping: "acme", Platform: "linux/amd64"}, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(bin) {
		t.Fatalf("size changed: %d → %d", len(bin), len(out))
	}
	if bytes.Equal(out, bin) || !bytes.HasPrefix(out, []byte("codecode")) || !bytes.HasSuffix(out, []byte("moremore")) {
		t.Fatal("only the slot may change")
	}

	meta, got, err := Decode(slotOf(t, out))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, mapping) || meta.Mapping != "acme" || len(meta.MappingSHA256) != 64 {
		t.Errorf("decoded %q, %+v", got, meta)
	}

	// Building again from a converter replaces the previous mapping entirely.
	again, err := Patch(out, Metadata{Mapping: "b"}, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if _, got, _ := Decode(slotOf(t, again)); string(got) != "x" {
		t.Errorf("re-patched mapping = %q", got)
	}
}

func TestDecodeDetectsDamage(t *testing.T) {
	out, _ := Patch(fakeBinary(), Metadata{Mapping: "acme"}, []byte(strings.Repeat("mapping ", 100)))
	clean := slotOf(t, out)
	used := bytes.LastIndexFunc(clean, func(r rune) bool { return r != 0 }) + 1
	// Any flipped byte, in the metadata, the mapping or the checksum, is caught.
	for i := headerSize; i < used; i++ {
		slot := bytes.Clone(clean)
		slot[i] ^= 0x01
		if _, _, err := Decode(slot); err == nil {
			t.Fatalf("byte %d flipped: decoded without error", i)
		}
	}
}

func TestPatchRejects(t *testing.T) {
	if _, err := Patch([]byte("no slot here"), Metadata{}, nil); err == nil {
		t.Error("binary without slot: want error")
	}
	twice := append(fakeBinary(), fakeBinary()...)
	if _, err := Patch(twice, Metadata{}, nil); err == nil {
		t.Error("binary with two slots: want error")
	}
	random := make([]byte, SlotSize) // incompressible: cannot fit
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range random {
		random[i] = byte(rng.Uint32())
	}
	if _, err := Patch(fakeBinary(), Metadata{}, random); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("huge mapping: err = %v", err)
	}
}
