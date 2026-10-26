# Golden test cases

One directory per case. The engine, the CLI and a built converter each run every case and must produce exactly the expected files.

```
testdata/<case-name>/
├── input.csv         source file, in the encoding and dialect under test
├── mapping.yaml      mapping to apply (may reference target.yaml in the same directory)
├── output.csv        expected output (absent when the conversion fails)
├── rejects.csv       expected reject file (absent if no row is rejected)
├── report.json       expected summary (hash and duration replaced by placeholders)
├── log.txt           expected log and error messages (paths and durations replaced)
├── exit_code         expected process exit code
├── inspect.txt       expected `lmmtfy inspect` output
└── init.yaml         expected `lmmtfy init` mapping (which must convert input.csv)
```

Cases run through `internal/app`, the same code path as the `lmmtfy` commands. Detection is also checked against each hand-written mapping: it must find the same input layout.

Comparison is byte for byte: encoding, BOM and line endings are part of the expectation.

- Name cases after what they test: `decimal-comma-thousands-space`, `optional-column-omitted`, `fail-file-on-bad-date`.
- Prefix a directory with `_` to park a case without deleting it.
- Regenerate expected files with `make golden-update`, then review the diff before committing.
