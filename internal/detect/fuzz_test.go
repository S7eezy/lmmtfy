package detect

import "testing"

// FuzzDetect: detection runs on files nobody has looked at; it must never
// panic, whatever the bytes.
func FuzzDetect(f *testing.F) {
	for _, s := range []string{"a;b\n1;2,5\nTotal;3\n", "x\n", "\xFF\xFEa\x00", "\"a\n", "1,2\n3\n4,5,6\n", ";;;\n;;;\n"} {
		f.Add([]byte(s), []byte(""), false)
	}
	f.Fuzz(func(t *testing.T, sample, tail []byte, truncated bool) {
		Detect(sample, tail, truncated, Options{SampleRows: 200})
	})
}
