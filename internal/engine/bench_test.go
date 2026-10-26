package engine

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"lmmtfy/internal/config"
)

// benchInput is a 200k-row file shaped like the skip-row-rejects case:
// integers, dates, decimals, value maps, strings.
func benchInput() string {
	var sb strings.Builder
	sb.WriteString("id,date,amount,status,country,comment\n")
	for i := 1; i <= 200000; i++ {
		fmt.Fprintf(&sb, "%d,%02d/%02d/2026,%d.%02d,%s,fr,comment number %d\n",
			i, i%28+1, i%12+1, i%9999, i%100, []string{"V", "A"}[i%2], i%1000)
	}
	return sb.String()
}

func BenchmarkRun(b *testing.B) {
	m, err := config.LoadMapping("../../testdata/skip-row-rejects/mapping.yaml")
	if err != nil {
		b.Fatal(err)
	}
	plan, issues := Compile(m)
	if plan == nil {
		b.Fatal(issues)
	}
	data := benchInput()

	for _, workers := range []int{1, 0} {
		name := fmt.Sprintf("workers=%d", workers)
		if workers == 0 {
			name = "workers=all"
		}
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				job := Job{Input: strings.NewReader(data), Output: io.Discard, Workers: workers}
				if _, err := plan.Run(job); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
