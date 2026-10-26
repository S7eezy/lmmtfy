// Package slot holds the payload slot of converter templates. Only the
// runner imports it: any binary importing it carries the 1 MiB slot.
package slot

import "lmmtfy/internal/payload"

// data is initialized, so the compiler stores all of it in the executable
// file, where `lmmtfy build` can overwrite it. This literal is the only
// place where the marker is written forwards.
var data = [payload.SlotSize]byte{'L', 'M', 'M', 'T', 'F', 'Y', '-', 'P', 'A', 'Y', 'L', 'O', 'A', 'D', '-', 'S', 'L', 'O', 'T', '-', 'v', '1'}

// Read returns the embedded mapping, or payload.ErrEmpty for a template.
func Read() (*payload.Metadata, []byte, error) {
	return payload.Decode(data[:])
}
