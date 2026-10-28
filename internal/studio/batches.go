package studio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/engine"
)

// BatchRows is the number of sample rows batch statistics read.
const BatchRows = 20000

// BatchStats describes the batches of a sample file for a key: what the
// Combine rows dialog shows before the user commits to it.
type BatchStats struct {
	Rows    int  `json:"rows"`
	Batches int  `json:"batches"`
	Min     int  `json:"min"` // rows in the smallest batch
	Max     int  `json:"max"`
	Sorted  bool `json:"sorted"` // the rows of each batch are next to each other
	// Partial: the file has more rows than were read.
	Partial bool `json:"partial"`
}

// Batches reads the start of a sample file, as the mapping reads it, and
// counts the batches the source columns by make.
func Batches(path string, m *config.Mapping, by []string) (*BatchStats, error) {
	m = clone(m)
	m.ApplyDefaults()
	in := m.Input
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := csvio.NewReader(f, csvio.ReaderOptions{
		Dialect:       csvio.Dialect{Delimiter: config.Char(in.Delimiter), Quote: config.Char(in.Quote), Escape: csvio.Escape(in.Escape)},
		Encoding:      in.Encoding,
		BOM:           string(in.BOM),
		SkipLines:     in.HeaderRow - 1,
		CommentPrefix: string(in.CommentPrefix),
	})
	if err != nil {
		return nil, err
	}
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("no header: %w", err)
	}
	header, _ := engine.NormalizeHeader(head.Fields)
	idx := make([]int, len(by))
	for i, name := range by {
		if idx[i] = slices.Index(header, name); idx[i] < 0 {
			return nil, fmt.Errorf("%q is not a column of the sample", name)
		}
	}
	for range in.SkipRowsAfterHeader {
		r.Read()
	}

	st := &BatchStats{Sorted: true}
	sizes := map[string]int{}
	var last string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if st.Rows == BatchRows {
			st.Partial = true
			break
		}
		st.Rows++
		parts := make([]string, len(idx))
		for i, j := range idx {
			if j < len(rec.Fields) {
				parts[i] = strings.TrimSpace(rec.Fields[j])
			}
		}
		key := strings.Join(parts, "\x00")
		if n, seen := sizes[key]; seen && key != last && n > 0 {
			st.Sorted = false
		}
		sizes[key]++
		last = key
	}
	st.Batches = len(sizes)
	for _, n := range sizes {
		if st.Min == 0 || n < st.Min {
			st.Min = n
		}
		st.Max = max(st.Max, n)
	}
	return st, nil
}
