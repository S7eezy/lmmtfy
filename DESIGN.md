# lmmtfy — Let Me Map That For You

Design document. Reference for all implementation work. Update it when a decision changes.

## 1. Purpose

Messy, heterogeneous CSV files arrive from many sources. A downstream software expects one fixed CSV format (exact columns, order, types, dialect). lmmtfy lets a user describe, once per source, how to turn that source into the expected format, then apply that description anywhere:

- in a desktop editor (lmmtfy Studio),
- from a CLI (`lmmtfy convert`),
- with a standalone, dependency-free converter binary generated from a single mapping (`acme-converter.exe`).

### Goals (v1)

- Open any CSV, tune reading parameters with a live preview.
- Auto-detect column types; rename, retype, delete, reorder, add columns (with defaults).
- Value mapping (`V` → `VALIDATED`).
- Optional columns with explicit behaviour when missing or empty.
- Configurable error strategy and logging.
- Stream files of several GB with constant memory.
- Export standalone converters for Windows, Linux and macOS, fully offline.
- Production is air-gapped: converters and the CLI may never require network access at build or run time.
- Studio may run on a connected PC, but must install and run **without admin rights**.

### Non-goals (v1)

- Expressions / computed columns (concat, split, arithmetic).
- Joins, lookups against other files, multi-file inputs.
- Non-CSV inputs or outputs (Excel, JSON, databases).

The config schema must leave room for these (versioned, extensible).

## 2. Concepts

| Concept | What it is | File |
|---|---|---|
| **Target** | The format the downstream software expects: ordered output columns, their types and constraints, and the output CSV dialect. Reusable across mappings. | `*.target.yaml` |
| **Mapping** | How one source format becomes a target: input dialect, source → target column rules, value maps, error and logging strategies. One mapping per source format; a new source variation means a new mapping. | `*.mapping.yaml` |
| **Engine** | The streaming pipeline that applies a mapping to a file. Shared by Studio, CLI and runner — one implementation, no drift. | Go package |
| **Runner** | A prebuilt minimal converter binary. A standalone converter is a runner with a resolved mapping appended to it. | `cmd/runner` |

A mapping references a target either by path or inline. A mapping with an inline target is **standalone**. In Studio, "Select target" offers existing target files or "New target from scratch" (created inline, can be extracted to a file later).

### Editor operations → config

| Editor action | Config effect |
|---|---|
| Rename column | Target column `name` differs from rule `source` |
| Change type | Target column `type` (+ mapping parse options) |
| Delete column | Source column not referenced by any rule (dropped) |
| Reorder | Order of target `columns` list |
| Add column | Target column with no `source`, filled by `default` |
| Value mapping | Rule `map` |

## 3. Architecture

### Technology

- **Go** (1.27+) for everything except the Studio frontend. Static binaries, trivial cross-compilation, streaming CSV, no runtime to install.
- **Wails** for Studio: Go backend + web frontend (Svelte + TypeScript) in a native desktop window.
- **YAML** for configs (`github.com/goccy/go-yaml`, chosen for error messages with line/column positions).
- `golang.org/x/text/encoding` for non-UTF-8 encodings.
- Fixed-point decimals (`github.com/shopspring/decimal` or in-house) — never floats for decimal data.

### Binaries

There are three binaries, deliberately separate:

| Binary | Contents | Why separate |
|---|---|---|
| `lmmtfy` | CLI: convert, validate, inspect, init, build. Embeds all runners. | Pure Go, static, runs on headless air-gapped servers. A Wails binary would require WebKitGTK on Linux. |
| `lmmtfy-studio` | Wails desktop GUI. Embeds all runners. | Needs a webview; built natively per OS. |
| `runner` | Minimal converter template. Never distributed alone. | Smallest possible artifact for production. |

Runners are pure Go and cross-compiled for every platform in one build step, then embedded (`go:embed`) into both `lmmtfy` and `lmmtfy-studio`. Any installation can therefore produce converters for any platform, offline.

Runner targets: `windows/amd64`, `linux/amd64`, `linux/arm64`, `darwin/arm64`, `darwin/amd64`.

### Repository layout

```
lmmtfy/
├── go.mod
├── Makefile                build runners → embed → build CLI / Studio
├── cmd/
│   ├── lmmtfy/             CLI entry point
│   └── runner/             standalone converter template
├── internal/
│   ├── config/             schema structs, load, validate, resolve (inline target)
│   ├── csvio/              streaming reader/writer, dialects, encodings
│   ├── types/              per-type parse / validate / format
│   ├── detect/             dialect and type detection
│   ├── engine/             pipeline
│   ├── report/             logging, reject file, summary
│   ├── payload/            append / extract mapping in runner binaries
│   └── runners/            embedded runner binaries (build output)
├── studio/                 Wails app (Go bindings + frontend/)
├── testdata/               golden test cases
└── DESIGN.md
```

## 4. Configuration

All files carry `version: 1`. The engine refuses an unknown major version; future migrations are explicit.

General rule: **the target describes the output representation; the mapping describes how to read the source.** E.g. the target says "decimal, scale 2, `.` separator"; the mapping says "source uses `,` as decimal separator and spaces for thousands".

### 4.1 Target file

```yaml
version: 1
name: erp-orders
description: Order import format for the ERP

output:
  encoding: utf-8           # utf-8 | utf-16le | utf-16be | latin-1 | windows-1252 | ...
  bom: false
  delimiter: ","
  quote: '"'
  quote_style: minimal      # minimal | always | never | non_numeric
  escape: double            # double ("" inside quotes) | backslash
  line_ending: crlf         # crlf | lf
  header: true
  null_value: ""            # how empty values are written

columns:                    # output order = list order
  - name: order_id
    type: integer
    required: true
    min: 1
  - name: order_date
    type: date
    format: "YYYY-MM-DD"
    required: true
  - name: amount
    type: decimal
    scale: 2
    rounding: half_up
    decimal_sep: "."
  - name: status
    type: string
    allowed_values: [VALIDATED, CANCELLED, PENDING]
  - name: country
    type: string
    max_length: 2
    case: upper
  - name: comment
    type: string
    max_length: 255
    on_overflow: truncate   # truncate | error
```

### 4.2 Mapping file

```yaml
version: 1
name: acme-orders
description: ACME weekly export

target: ./erp-orders.target.yaml   # path, relative to this file — or an inline target object

input:
  encoding: latin-1
  bom: auto                 # auto | true | false
  delimiter: ";"
  quote: '"'
  escape: double            # double | backslash | none
  header: true
  header_row: 3             # 1-based line of the header; lines above are skipped
  skip_rows_after_header: 0
  skip_footer: 1            # trailing lines to ignore
  comment_prefix: "#"       # lines starting with this are ignored
  null_tokens: ["NULL", "N/A", "-"]   # read as empty, like ""
  trim: true                # default false
  ragged_rows: error        # error | pad (short rows) | truncate (long rows) | lenient (both)
  duplicate_headers: suffix # error | suffix (Name, Name_2, ...)

on_error:
  strategy: skip_row        # fail_file | skip_row | default
  max_error_rate: 0.01      # fail the file if more than 1% of rows had errors (checked at end)
  max_errors: 1000          # fail early after N errors (optional)
  reject_file: true         # write bad rows to <output>.rejects.csv

logging:
  level: errors             # none | errors | warnings | info | trace
  format: text              # text | json
  file: null                # null = stderr
  summary: true

columns:                    # keyed by target column name; order comes from the target
  order_id:
    source: "N° Commande"
  order_date:
    source: "Date"
    formats: ["DD/MM/YYYY", "DD-MM-YYYY"]
  amount:
    source: "Montant TTC"
    decimal_sep: ","
    thousands_sep: " "
  status:
    source: "Etat"
    map: { V: VALIDATED, A: CANCELLED, E: PENDING }
    map_unmatched: error    # keep | error | default
  country:
    default: "FR"           # no source: added column with constant value
  comment:
    source: "Remarque"
    optional: true
    if_missing: empty       # error | empty | omit | default
    if_empty: default       # keep | default | error
    default: "-"
    on_error: default       # per-column override of on_error.strategy
```

A standalone mapping replaces `target: ./file` with an inline object holding `output:` and `columns:` exactly as in a target file.

### 4.3 Source references

- `source: "Header name"` — by header (exact match, after BOM removal and trimming).
- `source_index: 4` — by 1-based position, for files with `header: false`.
- No source — column is filled by `default` (required in that case).

### 4.4 Types

| Type | Target options (output) | Mapping options (input) |
|---|---|---|
| `string` | `max_length`, `on_overflow`, `case` (none/upper/lower), `pattern` (regex), `allowed_values` | `trim` |
| `integer` | `min`, `max`, `thousands_sep` | `thousands_sep` |
| `decimal` | `scale`, `rounding` (half_up/half_even/truncate), `decimal_sep`, `thousands_sep`, `min`, `max` | `decimal_sep`, `thousands_sep` |
| `boolean` | `true_value`, `false_value` (e.g. `1`/`0`) | `true_values`, `false_values` (lists, case-insensitive) |
| `date` | `format` | `formats` (list, tried in order) |
| `datetime` | `format`, `timezone` | `formats`, `timezone` (assumed for naive values) |
| `time` | `format` | `formats` |

Date tokens are human-readable and translated internally; users never see Go's reference-date layouts:

| Token | Meaning | Token | Meaning |
|---|---|---|---|
| `YYYY` / `YY` | year | `HH` / `hh` / `h` | hour 24h / 12h / 12h unpadded |
| `MM` / `M` | month (`M` accepts 1 or 2 digits) | `mm` / `m` | minutes |
| `MMM` / `MMMM` | `Jan` / `January` (English only) | `ss` / `s` | seconds |
| `DD` / `D` | day (`D` accepts 1 or 2 digits) | `.SSS` `.SSSSSS` | fractional seconds, after `.` or `,` |
| `A` / `a` | `PM` / `pm` | `Z` / `ZZ` | offset `+02:00` (or `Z`) / `+0200` |

Punctuation, spaces and `T` are literal. Other letters and digits are rejected, so a typo cannot silently become a wrong format.

Delimiters and quotes accept aliases for awkward characters: `tab`, `comma`, `semicolon`, `pipe`, `space`, and `quote: none` to disable quoting.

Scalars are read as written: `01` stays `01`, `1.50` stays `1.50`, `yes` stays `yes`. YAML's own rules still apply, so values starting with `-`, `|`, `*` or containing `: ` must be quoted.

Common to all types: `required`, `default`.

### 4.5 Optional columns

Two distinct situations:

- **Column missing from the input file** (decided once, when reading the header) → `if_missing`:
  - `error` (default for non-optional columns): file fails.
  - `empty`: column present in output, all values empty.
  - `default`: column present in output, all values = `default`.
  - `omit`: column removed from the output entirely.
- **Value empty in a row** (null token or empty after trim) → `if_empty`: `keep`, `default`, `error`.

When optional columns are missing, the engine emits one warning at the start of the conversion:

```
WARN  configuration has 3 optional columns missing from input: bla, bli, blo
      bla → empty, bli → default "-", blo → omitted from output
```

`omit` changes the output column set, which the target software may reject. Studio shows a warning when `omit` is selected, and `lmmtfy validate` reports it.

Precedence: mapping `default` overrides target `default`.

Defaults are written in the **output** representation (`default: "0,00"` for a target with `decimal_sep: ","`) and checked when the mapping is loaded. `default: ""` means an empty value.

### 4.6 Error strategies

Triggered by any cell-level failure (parse error, constraint violation, unmatched value map with `map_unmatched: error`, `if_empty: error`).

| Strategy | Effect |
|---|---|
| `fail_file` | Stop immediately. No output file is left behind. |
| `skip_row` | Row is not written; it goes to the reject file if enabled. |
| `default` | Faulty cell replaced by the column `default`; if the column has none, by empty — unless it is `required`, which is a config validation error. |

The global `on_error.strategy` can be overridden per column. `max_error_rate` and `max_errors` turn too many handled errors into a file failure.

Defaults (safest first; Studio asks for the strategy explicitly when creating a new mapping):

- `on_error.strategy: fail_file` — any row error fails the whole conversion.
- `on_error.reject_file: true` — only matters with `skip_row`: skipped rows are collected in `<output>.rejects.csv` instead of being silently lost.
- `max_error_rate` / `max_errors`: unset.

### 4.7 Logging and reporting

- Levels: `none`, `errors` (each error), `warnings` (+ missing optional columns, defaults applied, truncations), `info` (+ start/end, timings), `trace` (every row; for debugging only, slow).
- `text` for humans, `json` (one object per line) for log collectors.
- **Reject file**: same dialect as the input, original raw values, plus `_line`, `_column`, `_error` columns.
- **Summary** (end of run, and `--report report.json`): rows read, written, skipped, defaulted, per-column error counts, missing optional columns, duration, mapping name + hash, engine version.

## 5. Engine

### Pipeline

```
open → decode (encoding, BOM) → tokenize (dialect) → header resolution
     → for each row: cell pipeline per target column → write row | reject row
     → finalize (error-rate check, summary, atomic rename)
```

Header resolution happens once: resolve each rule's source to an index, apply `if_missing`, compute the effective output column list, emit missing-column warnings.

Cell pipeline, in order:

1. Extract raw value (by resolved index).
2. Trim (if enabled).
3. Null token check → empty.
4. Value map (exact match on the raw string; `map_unmatched` decides the rest). Mapped values are still parsed in step 6, so they are written in the input representation. `map_unmatched` without `map` is a configuration error.
5. Empty handling (`if_empty`).
6. Parse into the typed value using mapping input options.
7. Validate against target constraints.
8. Format to the target's output representation.

Failure at any step → the column's error strategy.

### Streaming and safety

- Row by row, constant memory, regardless of file size.
- Header names are trimmed before matching; a BOM never ends up in a header.
- Text after a closing quote, before the next delimiter, is kept as part of the field (`"ab"c` reads as `abc`). Real-world files do this.
- Custom CSV tokenizer: Go's `encoding/csv` only supports `"` as quote and no escape character. Benchmarked against `encoding/csv` for the default dialect.
- Output written to a temp file next to the destination, then renamed on success. A failed conversion never leaves a partial output.
- Rows are converted in parallel, results applied in input order (see below).
- Decimals use int64 arithmetic up to 18 significant digits and arbitrary precision beyond, or when min/max are set. Differential and fuzz tests check that both paths always agree.
- Target: 1 GB in well under a minute on a laptop, memory under 100 MB. Measured on a 10-core Apple laptop, 956 MB / 20.6 M rows: 7.3 s and 41 MB peak memory with all cores, 12 s and 16 MB with `--workers 1` (18 s before parallelism and the decimal fast path).

### Parallelism

```
reader ──batches of 1024 rows──▶ workers (one per core) ──▶ apply, in input order ──▶ writer
   └────────────── same batches, in input order ──────────────┘
```

- Workers convert and format rows; they only read the plan and write to their batch.
- One "apply" stage takes batches back in input order and does everything with shared state: counters, logs, error strategies, thresholds, reject file, output. Output, reject file, logs and summary are therefore identical whatever the number of workers; tests check this on every golden case with batches of 1 to 3 rows and 1 to 8 workers, under the race detector.
- At most 4 batches per worker are in flight, so memory stays bounded.
- `--workers N` limits the cores used (default: all). Reading and applying are single-threaded and set the ceiling.

### Detection (Studio, `lmmtfy inspect`, `lmmtfy init`)

Detection reads the first 4 MiB of the file (at most 10,000 data rows) and, for larger files, its last 64 KiB to find a footer.

- **Encoding**: BOM, then UTF-16 recognized by its zero bytes, then UTF-8 validity, else `windows-1252` (superset of latin-1).
- **Delimiter**: tab, `;`, `|`, `,` — the one giving the most consistent field count over the first 200 records, more fields on ties, then that order (`1;2,50` is two fields either way: `;` wins, since `,` is then most likely a decimal separator).
- **Header row**: the first record with the usual field count and more than half its cells filled. Records above (titles, export dates) are skipped with `header_row`.
- **No header**: the candidate header row is data when most typed columns parse it.
- **Footer**: up to 3 trailing records that do not fit: short rows with few values ("End of export"), rows most typed columns reject ("Total;3;;;"), or a rejected value in a row starting with "Total", "Sum", "End"… Types are inferred without the last records first, since a footer would spoil them.
- **Trim**, **ragged rows**, **duplicate headers**, **backslash escapes**: set when the sample shows them.
- **Types**, per column, on non-empty values: each candidate reading (integer, decimal with every separator combination, boolean pairs such as oui/non, 18 date formats, datetimes, times) is tried with the real type converters, so a proposal is guaranteed to convert the sample. The candidate accepting the most values wins, ties broken in that order; it must accept at least 95% of values, else the column is text. The 5% tolerance covers stray bad values, which are listed.
  - Thousands separators must be consistent (`1 234 567`): `12.34.5` is not an integer.
  - Numbers with leading zeros (`007`) stay text: they are codes.
  - Common null tokens (`NULL`, `N/A`, `-`…) in an otherwise typed column become `null_tokens`.
  - Several date formats in one column are covered with up to 3 formats.
- **Notes** flag what a human should check: ambiguous day/month order, `.`/`,` read as thousands separators, 8-digit integers that are also valid dates, text columns that almost parse ("80% of values read as date DD/MM/YYYY; rejected: 31/02/2026").
- Target names are derived from headers: lower-case ASCII, accents removed, underscores (`N° Commande` → `n_commande`); `column_N` without header.

`lmmtfy init` writes a standalone mapping from the result: input options as detected, an inline target with normalized output (UTF-8, ISO dates, `.` decimals, scale from the sample) for the user to adapt, one rule per column with a comment describing what was seen, and `REVIEW:` comments for notes. The generated mapping is compiled before it is written.

Detection only proposes; the user decides.

## 6. CLI

```
lmmtfy convert <input.csv> --mapping <m.yaml> -o <out.csv> [--rejects path] [--report r.json] [--log-level L] [--strict] [--workers N]
lmmtfy validate --mapping <m.yaml> [--input <input.csv>]     # check config; optionally check an input header against it
lmmtfy inspect <input.csv> [--sample N] [--json]             # detected dialect, columns, types
lmmtfy init <input.csv> -o <m.yaml> [--name N] [--force]     # draft mapping from detection (standalone)
lmmtfy build --mapping <m.yaml> --os windows|linux|darwin [--arch amd64|arm64] -o <converter>
lmmtfy version
```

Exit codes:

| Code | Meaning |
|---|---|
| 0 | Success (handled errors allowed, unless `--strict`) |
| 1 | Success with handled errors, `--strict` only |
| 2 | Usage or configuration error |
| 3 | File failed (`fail_file`, `max_error_rate`, `max_errors`, missing required column) |
| 4 | I/O error |

## 7. Standalone converters

### Build

`lmmtfy build` (or Studio "Export converter"):

1. Load the mapping, resolve the target (inline it), validate.
2. Serialize the resolved mapping, gzip it.
3. Copy the embedded runner for the chosen OS/arch and write the payload into its **payload slot**.
4. macOS only: regenerate the ad-hoc code signature.

The payload slot is a reserved, initialized 1 MiB byte array compiled into the runner (a Go package-level variable starting with a unique marker). Because it is initialized, it lives in the binary's data section, so `build` can find the marker and overwrite the slot in place without changing the file layout. Mappings are a few KB; 1 MiB is far more than enough, and `build` refuses a payload that does not fit.

Slot layout:

```
["LMMTFY-SLOT\x00\x01"][u32 metadata length][metadata JSON][u32 payload length][gzip(resolved mapping YAML)][zero padding…]
```

Metadata: mapping name, SHA-256 of the resolved mapping, engine version, build date.

At startup the runner reads the slot variable directly from memory (no file access to its own executable), validates the mapping. Empty slot → refuses to run.

Why a slot rather than appending data at the end of the file: the file structure stays valid on every OS, and on macOS the signature can be regenerated over a well-formed binary.

### Usage

```
acme-converter <input.csv> <output.csv> [--rejects path] [--report r.json] [--log-level L] [--strict]
acme-converter --info            # mapping name, hash, engine version, build date
acme-converter --print-config    # embedded mapping, for audit
```

Same exit codes as the CLI. The mapping cannot be changed at run time; that is the point.

### Platform notes

- **Windows**: runners are unsigned. If Authenticode signing is ever required, sign the converter *after* `build` (patching the slot would invalidate an earlier signature).
- **Linux**: static binary, no dependencies.
- **macOS**: Apple Silicon refuses to run any unsigned binary, and patching the slot invalidates the signature the Go linker applied. `build` therefore regenerates the ad-hoc signature itself, in pure Go (code-signature hashing ported from the Go toolchain's own `cmd/internal/codesign`, BSD-licensed). This works from any OS: a Linux or Windows machine can produce a working macOS converter. Converters are not notarized; copied over a network share or USB they run directly, downloaded through a browser they carry the quarantine flag and need a right-click → Open the first time.

## 8. Studio (GUI)

1. **Open** a CSV.
2. **Reading parameters**: detected values pre-filled (encoding, delimiter, quote, escape, header row, skip rows, footer, comment prefix, null tokens). Live preview of the first 100 rows, updating on every change.
3. **Editor**:
   - Target picker: existing target file, or new target from scratch (inline).
   - Column list: source name, detected type + confidence, target name, target type, sample values, status (mapped / dropped / added / error).
   - Per column: rename, retype + type options, delete, drag to reorder, value map table, optional / missing / empty behaviour, default, error override.
   - Add column: name, type, default.
   - Live output preview on the sample, with error cells highlighted.
   - Warnings panel (unmapped required target columns, `omit` used, type confidence low, …).
4. **Save** mapping (and target, if extracted).
5. **Convert**: open CSV → pick mapping → run with progress, summary, link to rejects.
6. **Export converter**: pick mapping → OS/arch → save binary.

Studio calls the same engine package as the CLI; previews run the real pipeline on the sample.

### Packaging: no admin rights

Studio and the CLI ship as **portable archives**: unzip anywhere (home folder, USB stick) and run. No installer, no registry, no admin.

| OS | Studio | Admin needed? |
|---|---|---|
| Windows | Single `lmmtfy-studio.exe`. Uses the Edge WebView2 runtime, preinstalled on Windows 11 and on up-to-date Windows 10. If absent, the archive variant `-webview2` carries a fixed-version runtime in a side folder, used without installation. | No |
| Linux | Binary needing WebKitGTK, present on standard desktops (GNOME, KDE). An AppImage bundling it is the fallback if needed. | No (unless WebKitGTK is missing and must be installed system-wide) |
| macOS | `lmmtfy-studio.app`, drag anywhere. | No |

The CLI is a single static binary on every OS. Settings and recent files live in the user's config directory (`os.UserConfigDir()`), never next to the executable or in system paths.

## 9. Testing

- Unit tests per type parser/formatter and per tokenizer feature.
- **Golden tests**: `testdata/<case>/` holds `input.csv`, `mapping.yaml`, expected `output.csv`, `rejects.csv`, `report.json`. Run through the engine, the CLI and a built runner; all three must produce identical results.
- Fuzzing of the CSV tokenizer and type parsers (`go test -fuzz`).
- Generated multi-GB benchmark file for throughput and memory checks.
- CI builds the full matrix of runners on every change.

## 10. Milestones

| # | Scope | Done when |
|---|---|---|
| M0 | Repo scaffold, Makefile, CI, test harness | `make test` green, runners cross-compile |
| M1 | Config load/validate, types, CSV I/O, engine, `convert`, `validate` | Golden tests pass, GB file streams in constant memory |
| M2 | Detection, `inspect`, `init` | Draft mapping generated from real sample files |
| M3 | Runner, payload slot, macOS re-signing, `build` | Converters for Windows, Linux and macOS match CLI output on all golden tests |
| M4 | Studio: open, preview, editor, save, convert, export | End-to-end flow without touching YAML |
| M5 | Hardening: performance, docs, portable packaging | Portable archives run without admin rights on all three OS |

## 11. Open questions

- Frontend framework for Studio: Svelte proposed.
- Header matching: exact only, or optionally case-insensitive / whitespace-insensitive?
- Expressions (concat, split, arithmetic) for v2: syntax to choose.
