package config

import "testing"

// FuzzParseMapping: any document must load or fail cleanly, never panic,
// and a loaded one must survive defaults and validation.
func FuzzParseMapping(f *testing.F) {
	f.Add([]byte("version: 1\nname: m\ntarget:\n  columns: [{name: a, type: string}]\ncolumns:\n  a: {source: A, map: {01: x}}\n"))
	f.Add([]byte("target: t.yaml\ninput: {delimiter: tab, null_tokens: [~, '']}\n"))
	f.Add([]byte("columns: {a: {default: [1, 2]}}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := ParseMapping(data)
		if err != nil {
			return
		}
		m.ApplyDefaults()
		Validate(m)
		Marshal(m)
	})
}
