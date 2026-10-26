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
| 130 | Interrupted (Ctrl+C, SIGTERM): temporary files removed, no output written |

## 7. Standalone converters

### Build

```
lmmtfy build --mapping acme.mapping.yaml --os windows|linux|darwin [--arch amd64|arm64] -o acme-converter
```

`--arch` defaults to amd64, arm64 for darwin; `.exe` is added for Windows. Studio's "Export converter" does the same.

1. Load the mapping, resolve the target (inline it), validate. An invalid mapping never becomes a converter.
2. Serialize the resolved mapping and gzip it.
3. Take the embedded runner template for the chosen platform and write the payload into its **payload slot**.
4. macOS: regenerate the ad-hoc code signature, then verify it.

The payload slot is a reserved, initialized 1 MiB byte array compiled into the runner (package `internal/payload/slot`, imported only by the runner). Because it is initialized, the compiler stores all of it in the executable file, so `build` can find its marker and overwrite the slot in place without changing the file layout. The marker is written forwards only in the slot's own initializer (other code spells it backwards), and `build` refuses a template where it does not appear exactly once. Mappings are a few KB compressed; `build` refuses one that does not fit.

Slot layout (lengths little-endian):

```
[marker "LMMTFY-PAYLOAD-SLOT-v1"][2 zero bytes][u32 metadata length][metadata JSON]
[u32 mapping length][gzip(resolved mapping YAML)][SHA-256 of the fields since the marker][zero padding…]
```

Metadata: mapping name, SHA-256 of the resolved mapping, engine version, platform, build date, building lmmtfy version.

At startup the converter reads the slot from memory (it never opens its own executable), checks the checksum, decompresses and compiles the mapping. An empty slot (the bare template) or a damaged one refuses to run, with exit code 2.

Why a slot rather than appending data at the end of the file: the file structure stays valid on every OS, and on macOS the signature can be regenerated over a well-formed binary.

Templates are embedded gzipped in `lmmtfy` (about 2 MB each instead of 6), so the CLI carrying all five platforms weighs about 17 MB. `internal/build` is a separate package from `internal/app` for that reason: the runner uses `app`, and must not contain the other runners.

### Usage

```
acme-converter <input.csv> <output.csv> [--rejects path] [--report r.json] [--log-level L] [--strict] [--workers N]
acme-converter --info            # mapping name, hash, engine version, platform, build date
acme-converter --print-config    # embedded mapping, for audit
```

Same engine, messages and exit codes as `lmmtfy convert` (the code is shared, in `internal/app`). Messages are prefixed with the converter's file name. The mapping cannot be changed at run time; that is the point.

Tests build a real converter for every golden case and run it as a separate process: output, reject file, report, log and exit code must equal the CLI's expected files. Converters for all five platforms are built in tests, decoded back, and their signatures verified.

### Platform notes

- **Windows**: converters are unsigned. If Authenticode signing is ever required, sign the converter *after* `build` (patching the slot would invalidate an earlier signature).
- **Linux**: static binary, no dependencies.
- **macOS**: Apple Silicon refuses to run a binary whose code signature does not match its content, and patching the slot changes some pages. The Go linker signs darwin/arm64 binaries "ad hoc": a code directory holding the SHA-256 of every 4 KiB page. `build` recomputes those hashes in pure Go (the signature keeps its size and layout), so any OS can produce a working macOS converter; `codesign --verify --strict` accepts the result. A converter signed with a real certificate after building would have to be signed again if rebuilt. darwin/amd64 binaries are not signed by the Go linker, which Intel Macs accept. Converters are not notarized: copied over a network share or USB they run directly; downloaded through a browser they carry the quarantine flag and need right-click → Open the first time.

## 8. Studio (GUI)

### Flow

1. **Home**: new mapping from a CSV file, or open a mapping.
2. **Read the file**: detection pre-fills encoding, delimiter, quote, escape, header line, rows to skip, footer, comments, trim, null tokens, ragged rows and duplicate headers. The preview shows the first rows, marked as skipped, header, data or footer, and follows every change. "Detect again" resets to detection.
3. **Map columns**:
   - Output columns, in output order: name, type, source column (or none: filled with the default). Move up/down, delete, add. Source columns not used are listed and can be added back in one click.
   - Details of the selected column: what detection saw (type, share of values, examples, notes), required, default, empty and missing behaviour (optional columns, with the omit warning), error strategy override, trim, type options (how values are read and written), value map.
   - Live output preview of the first 100 rows, with rejected rows and messages in their own tabs. When the mapping is invalid, the problems replace the preview, the column carries a marker, and Save, Convert and Export are disabled.
   - Settings: mapping name and description, where the target lives (in the mapping, in its own file, or an existing target file shared with other mappings), output file format, error strategy and thresholds, logging.
4. **Save** (YAML, rules in output order, target inline or as a relative path), **Convert…** a whole file, **Export converter…** for any platform.

Opening a saved mapping goes straight to step 3; a sample CSV can then be opened for the preview.

### Architecture

- `internal/studio` (root module): all the logic, stateless — open/detect, raw preview, check, output preview (the real engine on the first 100 rows, in memory), load/save, convert, export. The editor sends the mapping being edited with each call, as JSON; `config` types carry JSON tags for that. Unit-tested like the rest.
- `studio/` (separate Go module): the Wails app. `app.go` only forwards calls to `internal/studio` and opens native file dialogs. Being a separate module keeps `go test ./...` and the CLI free of cgo and web views. The Wails CLI is a Go tool dependency of that module (`go tool wails`), so nothing is installed globally.
- `studio/frontend`: Svelte 5 + Vite, plain JavaScript (no TypeScript build step). Bindings in `frontend/wailsjs` are generated by Wails from `app.go`.
- `make studio` builds the app for this machine, `make studio-windows` cross-compiles the Windows one from macOS or Linux. Both embed the converter templates, so Studio exports converters offline.

### Packaging: no admin rights

`make dist` builds portable archives into `dist/release/` (`tools/release`, in Go so it behaves the same on every OS): unpack anywhere and run. No installer, no registry, no admin.

| Archive | Contents |
|---|---|
| `lmmtfy-<version>-<os>-<arch>` (.zip for Windows, .tar.gz otherwise; 5 platforms) | CLI with all converter templates, README.txt, USER_GUIDE.md, the acme example |
| `lmmtfy-studio-<version>-windows-amd64.zip` | Single `lmmtfy-studio.exe` (cross-compiled from macOS or Linux) |
| `lmmtfy-studio-<version>-macos.zip` | `lmmtfy Studio.app`, universal (Intel and Apple Silicon), ad-hoc signed; built on macOS |
| `lmmtfy-studio-<version>-linux-amd64.tar.gz` | Built on Linux (WebKitGTK 4.1) |
| `SHA256SUMS` | Checksums, to verify copies made to air-gapped machines |

Archive entries carry a fixed date, so their content alone decides their bytes.

- **WebView2 on Windows**: preinstalled on Windows 11 and up-to-date Windows 10. Where it is missing and cannot be installed, Microsoft's Fixed Version runtime unpacked in a `webview2` folder next to `lmmtfy-studio.exe` is used without installation (Studio checks for the folder at startup). The runtime itself is not bundled: it is Microsoft's download.
- **Linux**: Studio needs WebKitGTK, present on standard desktops.
- **Settings**: Studio's recent files live in the user's config directory (`os.UserConfigDir()/lmmtfy/studio.json`), never next to the executable or in system paths.
- **Icon**: drawn by `tools/appicon` (`go run ./tools/appicon -o studio/build/appicon.png`); Wails derives the macOS and Windows icons from it.
- Unsigned programs: Windows SmartScreen and macOS Gatekeeper (for browser downloads) ask for confirmation the first time; the user guide explains it. Organizations can sign the files with their own certificates (converters after building them).

## 9. Testing

- Unit tests per type parser/formatter and per tokenizer feature.
- **Golden tests**: `testdata/<case>/` holds `input.csv`, `mapping.yaml`, expected `output.csv`, `rejects.csv`, `report.json`. Run through the engine, the CLI and a built runner; all three must produce identical results.
- Fuzzing (`go test -fuzz`) of the CSV reader (no panic, write/read round trip), mapping parsing, detection and the decimal fast path. Inputs that once failed stay in `testdata/fuzz/` as regression tests. Fuzzing found: files shorter than 3 bytes failing to open, a panic inside the YAML library on malformed tags (now recovered and reported as an invalid document).
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
