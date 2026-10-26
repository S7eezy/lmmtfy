# Golden test cases

One directory per case. The engine, the CLI and a built converter each run every case and must produce exactly the expected files.

```
testdata/<case-name>/
├── input.csv         source file, in the encoding and dialect under test
├── mapping.yaml      mapping to apply (may reference a target file in the same directory)
├── output.csv        expected output
├── rejects.csv       expected reject file (absent if no row is rejected)
└── report.json       expected summary (durations excluded)
```

Comparison is byte for byte: encoding, BOM and line endings are part of the expectation.

- Name cases after what they test: `decimal-comma-thousands-space`, `optional-column-omitted`, `fail-file-on-bad-date`.
- Prefix a directory with `_` to park a case without deleting it.
- Regenerate expected files with `make golden-update`, then review the diff before committing.
