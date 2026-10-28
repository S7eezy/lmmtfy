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

- Expressions / computed columns (concat, split, arithmetic): done in v1.1, see §11.
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

A mapping references a target either by path or inline. A mapping with an inline target is **standalone**. A target file is a contract shared by several mappings: Studio edits it only in its target editor, never as a side effect of editing a mapping (see §8).

### Editor operations → config

| Editor action | Config effect |
|---|---|
| Rename column | Target column `name` differs from rule `source` |
| Change type | Target column `type` (+ mapping parse options) |
| Delete column | Source column not referenced by any rule (dropped) |
| Reorder | Order of target `columns` list |
| Add column | Target column with no `source`, filled by `default` |
| Value mapping | Rule `map` |
| Computed value (v1.1) | Rule `formula`, `cases`, `sequence` or `default_formula` (§11) |

## 3. Architecture

### Technology

- **Go** (1.27+) for everything except the Studio frontend. Static binaries, trivial cross-compilation, streaming CSV, no runtime to install.
- **Wails** for Studio: Go backend + web frontend (Svelte 5, plain JavaScript, Lucide icons) in a native desktop window.
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
├── Makefile                build runners → embed → CLI, Studio, release archives
├── cmd/
│   ├── lmmtfy/             CLI entry point
│   └── runner/             standalone converter template
├── internal/
│   ├── config/             schemas, loading, validation, YAML formatting
│   ├── csvio/              streaming CSV reader/writer, dialects, encodings
│   ├── types/              per-type parse / check / format
│   ├── detect/             encoding, dialect, layout and type detection
│   ├── engine/             parallel conversion pipeline
│   ├── report/             logs, run summary
│   ├── app/                commands shared by the CLI and converters
│   ├── build/              converter builds (embeds the templates)
│   ├── payload/            payload slot, macOS re-signing
│   ├── runners/            embedded converter templates (build output)
│   ├── studio/             Studio logic, without UI
│   ├── golden/             golden test harness
│   ├── exitcode/, version/
├── studio/                 Wails app (separate Go module) and its Svelte frontend
├── tools/                  icon generator, release packager
├── testdata/               golden test cases
├── docs/USER_GUIDE.md
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
| `decimal` | `scale`, `rounding` (half_up/half_even/truncate), `decimal_sep`, `thousands_sep`, `min`, `max` | `decimal_sep`, `thousands_sep`; scientific notation (`5.4e-07`, exponent up to ±308) is accepted, output is always plain notation |
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
7. Formula, if any (v1.1, §11.2).
8. Validate against target constraints.
9. Format to the target's output representation.

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
  - Decimal columns with scientific notation get no scale (the number of decimals varies with the exponent, a fixed scale would round tiny values to zero), and a note.
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

### Home

Two kinds of files, side by side, each with its recent files:

- **Mappings**: new mapping from a CSV file, open a mapping.
- **Targets**: new target, open a target.

Dropping a file on the window opens it: a CSV starts a new mapping, a `*.target.yaml` (or any YAML holding a target) opens the target editor, other YAML files open as mappings.

### Target rules

A mapping's target is in one of three states:

| State | Where it lives | In the mapping editor |
|---|---|---|
| **Linked** | An existing target file | Locked: output names, order, types, output formats, constraints and the output file format come from the file. The mapping only says how to fill each column: source, how the source is read, value map, default, error handling. |
| **New file** | Defined while creating the mapping, written to its own file on the first save | Editable, like an inline target. Once saved it is a file like any other: linked, locked. |
| **Inside the mapping** | Inline in the mapping file (one-off) | Editable. "Move to its own file" turns it into a new file. |

Locked does not mean stuck: **Edit target…** opens the target editor on the linked file, with a banner saying that saving changes every mapping using it. Back in the mapping, the target is reloaded, columns renamed in the editor are renamed in the mapping's rules too, and the Review list shows what changed (new columns to map, rules whose column is gone). **Copy as a new target file** forks the target for a source that needs a variant.

### New mapping

1. **Read the file**: detection pre-fills encoding, delimiter, quote, escape, header line, rows to skip, footer, comments, trim, null tokens, ragged rows and duplicate headers. The preview marks each row as skipped, header, data or footer, and follows every change.
2. **Target**, the main decision, as three cards:
   - **Use a target file**: recent targets or browse. The card shows the target (columns, required columns, output format) and how well this file fits it: target columns found by name, required ones missing.
   - **New target file** (selected by default): the target is drafted from this file's columns and saved next to the mapping as `<name>.target.yaml`, reusable by later mappings. Name, location and output file format are set here.
   - **Inside this mapping**: one-off, for a mapping nobody will share.
   Switching between cards keeps the draft, so trying a target file costs nothing. Opening the Target step of an existing mapping works the same way: "New target file" then means a copy of the current target.
3. **Settings**: mapping name, description, invalid values strategy, logging. Doing it before the columns means neither the name nor the strategy is forgotten.
4. **Map columns**:
   - One row per output column, read left to right: source column with its detected type and confidence (coloured), then output name and type. With an editable target, rows are reordered by dragging (auto-scroll near the edges, arrow keys on the grip), filtered by name, deleted or added; unused source columns can be added back in one click. With a linked target, names, types and order are shown but not editable, and a banner names the target file with **Edit target…**. Built for hundreds of columns.
   - Changing a type picks up the formats and separators detection found for it. A warning shows when the type does not match the data.
   - The selected column's panel: detection (confidence bar, examples, rejected values, notes), how the source is read, value map, how the output is written, invalid values.
   - **Review** list, first tab, in front whenever something needs a decision: target columns with no source (errors when required), source columns missing from the file, columns matched to a target by name (to confirm one by one or all at once), types that do not match the data, rules for a column the target no longer has (one click removes them), unused source columns. Each item jumps to its column; a target column without source can be given one directly from its panel.
   - Live output preview of the first 100 rows, selected column highlighted, with rejected rows and messages in their own tabs. When the mapping is invalid, the problems replace the preview and Save, Convert and Export are disabled.
5. **Save** (`Ctrl/⌘+S`), **Convert** a whole file, **Export** converters for any platform. Closing the window with unsaved changes asks first. Saving writes the target file only when the target is a new file; a linked target file is never rewritten by saving a mapping.

Opening a saved mapping goes straight to the columns; a sample CSV can then be opened for the preview. Every step stays reachable from the stepper, the Target step included.

### Target editor

- **New target** asks for a starting point: **from an example of the expected file** (a file the receiving software exports or documents, not a partner file) or **blank**. From an example, detection drafts the columns with the exact header names, their types, and how the example writes them: delimiter, encoding, line endings, date formats, decimal and thousands separators. That is the format to produce.
- **Open a target** edits an existing file: its name, description, output file format, and columns (name, type, required, default, constraints, output format), reordered by dragging like in the mapping editor. A column drafted from an example keeps its detected type, confidence and examples in its panel.
- Problems are checked as in mappings; a target with errors cannot be saved.
- Renaming or removing a column warns that mappings refer to columns by name. Studio cannot know every mapping using a target; the open mapping, if the editor was opened from it, is updated.
- **New mapping for this target** starts a new mapping with this target already chosen.

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

Releases are published by `.github/workflows/release.yml` when a `v*` tag is pushed: a Linux runner checks, smoke-tests and runs `make dist` (CLI for every platform, Studio for Windows and Linux), a macOS runner builds the universal Studio app (`make studio-macos`), and a last job publishes all archives as a GitHub Release with one `SHA256SUMS`. A tag with a suffix (`v1.1.0-rc1`) makes a pre-release.

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

All done; kept as the history of how lmmtfy was built.

| # | Scope | Done when |
|---|---|---|
| M0 | Repo scaffold, Makefile, CI, test harness | `make test` green, runners cross-compile |
| M1 | Config load/validate, types, CSV I/O, engine, `convert`, `validate` | Golden tests pass, GB file streams in constant memory |
| M2 | Detection, `inspect`, `init` | Draft mapping generated from real sample files |
| M3 | Runner, payload slot, macOS re-signing, `build` | Converters for Windows, Linux and macOS match CLI output on all golden tests |
| M4 | Studio: open, preview, editor, save, convert, export | End-to-end flow without touching YAML |
| M5 | Hardening: performance, docs, portable packaging | Portable archives run without admin rights on all three OS |

## 11. Operations (v1.1)

Status: implemented (M6 to M9). Ships as v1.1, once v1.0 is released.

Operations compute output values instead of copying them: row numbers, formulas, conditions, defaults computed from the whole file, and several input rows combined into one output row.

### 11.1 Filling a column

A rule fills its target column in one of five ways. Studio's column panel shows the choice as **Fill with**:

| Fill with | Rule keys | Example |
|---|---|---|
| Source column | `source`, optionally `formula` | `source: "Duration (ms)"`, `formula: "value / 1000"` |
| Fixed value | `default` | `default: FR` |
| Row number | `sequence` | `sequence: { start: 1 }` |
| Conditions | `cases`, `else` | §11.5 |
| Formula | `formula` | `formula: "[price] * [qty]"` |

`map`, `cases` and `formula` are one family, from exact lookup to anything. Studio offers the simplest that fits, and any of them can be opened as a formula.

Validation: `sequence` and `cases` exclude `source` and `formula`; a `formula` without `source` cannot use `value`.

**Helper columns** are read and typed like columns but never written. A condition or formula on a source column that is not in the output goes through a helper, so formulas never parse raw text themselves:

```yaml
helpers:
  temp:
    source: "Temp °C"
    type: decimal
    decimal_sep: ","
```

A helper is a rule plus a `type`. Helpers and target columns share one namespace; a clash is a validation error. A helper is written nowhere, but has the plain representation of its type (`1234.5`, `2024-01-31`, `2024-01-31T10:00:00` in UTC, `true`), which its `default` uses too. In grouped mode (§11.6), helpers have one value per input row.

### 11.2 Formula language

A small, Excel-flavoured language, implemented in `internal/expr` (lexer, parser, type checker, evaluator) and shared by the CLI, converters and Studio. Existing Go expression libraries compute in float64, which breaks money and unit conversions; formulas use the engine's exact decimals.

| Element | Syntax |
|---|---|
| Column | `[order date]`: a target or helper column, typed, **as written**: rounded to its scale, cased, defaulted. A value that must not be altered goes through a helper. `value`: this rule's source value, read as the column's type with the rule's parse options. |
| Literals | `12`, `1.5`, `"text"`, `true`, `false`, `date("2024-01-31")`, `datetime("2024-01-31T08:00:00Z")` |
| Arithmetic | `+ - * / %`, unary `-` |
| Text | `&` joins; an empty value joins as `""` |
| Comparison | `= <> < <= > >=` |
| Logic | `and`, `or`, `not` |

Functions (names are case-insensitive):

| Group | Functions |
|---|---|
| Logic | `if(cond, a, b)`, `ifs(cond1, a1, cond2, a2, …, else)`, `coalesce(a, b, …)`, `is_empty(x)` |
| Numbers | `round(x, digits)`, `floor`, `ceil`, `abs`, `min(a, b, …)`, `max(a, b, …)`, `number(text)` |
| Text | `upper`, `lower`, `trim`, `len`, `left(t, n)`, `right(t, n)`, `mid(t, start, n)`, `replace(t, find, with)`, `pad_left(t, n, char)`, `split_part(t, sep, n)`, `text(x)` |
| Dates | `add_days`, `add_months`, `days_between(a, b)`, `year`, `month`, `day`, `hour`, `minute`, `from_epoch(x, "s"\|"ms"\|"us")`, `to_epoch(x, unit)`, `today()`, `now()` |
| Row | `row()`: 1-based data row number in the input file |
| Aggregates | `first(x)`, `last(x)`, `lowest(x)`, `highest(x)`, `total(x)`, `mean(x)`, `count()`, `count(x)` |

Semantics:

- **Types** are the target types plus empty. Integer and decimal mix into decimal; `/` always gives a decimal, exact to 34 decimal places and rounded by the target column's `scale` and `rounding` when written.
- **Empty**: an empty cell and the text `""` are the same value. Arithmetic, number and date functions on an empty value return empty; text functions and `&` read it as `""`; `min` and `max` skip it. A comparison with an empty value is false, and an empty condition counts as false.
- **Defaults are final**: when a default fills the cell (empty source with `if_empty: default`, missing column with `if_missing: default`, unmatched value, `on_error: default`), the formula does not run. It runs on an empty source value otherwise, so `coalesce(value, [other])` works. An empty result is written empty, or is a row error when the column is `required`.
- **Checked at load**: unknown columns or functions, wrong argument types, and a result type the target column cannot take are validation errors with the position in the formula. Integer widens to decimal and anything converts to string; other mismatches are errors (`value / 1000` into an integer column: "the formula gives a decimal: use round(…)").
- **Run-time failures** (division by zero, invalid date, overflow) are cell errors handled by the column's `on_error`.
- After the formula, the target column's constraints and formatting apply as for any value.
- **Total and deterministic**: no loops, no file or network access, no user-defined functions. `today()` and `now()` are the time the conversion started, the same for every row; Studio marks formulas using them as changing from run to run.
- **Dependencies**: formulas may reference other columns and are evaluated in dependency order; a cycle is a validation error that names it. When a column fails and rejects the row, formulas reading it are not evaluated: the row has one error, not a cascade.
- `if(cond, a)` without else gives empty when the condition is false. `row()` is the data row number (1 = first row after the header).
- **Aggregates** range over the whole file, or over the current batch in grouped mode. Their argument can be a formula (`total([price] * [qty])`) but not another aggregate, nor `value`. Empty and unparsable values are left out (unparsable ones counted in a warning); with no values left, the result is empty (`count` gives 0, `count()` counts rows). `first` and `last` are the first and last values that are not empty. `lowest` and `highest` on text compare alphabetically. `mean` gives a decimal.

The cell pipeline of §5 gains one step: after parsing, the formula runs with `value` bound to the parsed value; validation and formatting follow.

**Studio formula field**: autocomplete for columns, helpers, source columns (picking one creates a helper, typed as most of its values read) and functions with their signature; errors with the offending character highlighted, checked as the formula is typed; the result type; examples (unit conversion, rounding, case, replace, Unix timestamp → datetime through an integer helper); and a live table of the first sample rows: the source value, the columns the formula reads and the result, errors in red.

### 11.3 Row numbers

```yaml
line_no:
  sequence: { start: 1, step: 1 }
```

- Counts **written** rows: a skipped row does not use up a number.
- Numbers are assigned in the ordered apply stage, so output stays identical whatever the number of workers. Formulas therefore cannot read the sequence; `row()` (the input row number) is available to them.
- In grouped mode, one number per output row.
- The target column must be an integer, a decimal or a string.

### 11.4 Defaults computed from the file

```yaml
start_date:
  default_formula: 'lowest([reading_date])'
```

`default_formula` replaces `default` (one or the other) wherever a default is used: no source, `if_empty: default`, `if_missing: default`, `on_error: default`, `map_unmatched: default`. Studio's default field gets a mode: Fixed value, First value of a column, Lowest or earliest of a column, Highest or latest of a column, Formula.

The same aggregates work inside any formula (`[price] / total([price])`).

- Aggregates need a **first pass** over the file before converting. It computes only the columns the aggregates read, in constant memory; reading takes about twice as long. With only `first` aggregates, it stops as soon as each has a value.
- A default formula reads columns inside aggregates only, so it gives one value per file; `today()` and `now()` are allowed.
- An aggregate cannot read a column computed from an aggregate: one pass must be enough. A column's own default formula does not count, since the first pass leaves out the cells a default would fill: `read_at` can default to `lowest([read_at])`.
- Input that cannot be read twice (a pipe) is copied to a temporary file first.
- Previews limit aggregates to the previewed rows; Studio's **Aggregates: whole file** switch computes them over the whole sample file. Conversions always use the whole file, and log each aggregate's value at level `info`.

### 11.5 Conditions

```yaml
temp_status:
  cases:
    - when: "[temp] > 30"
      then: HOT
    - when: "[temp] > 5"
      then: MILD
  else: COLD
```

- `when` is a formula giving a boolean. The first matching case wins.
- `then` and `else` are values in the output representation, checked at load like defaults; `then_formula` and `else_formula` compute them instead.
- No match and no `else`: empty, or a row error when the column is `required`.
- Studio builder: one block per case, `[column] [operator] [value]` (operators: is, is not, <, ≤, >, ≥, is empty, is not empty) then a fixed value or a formula; cases can be reordered; an Otherwise line. A source column picked in the builder becomes a helper. Any other condition (`and`, `or`, functions) is edited as formula text, with the formula field's checks.

### 11.6 Combining rows

Several input rows become one output row. Example: orders by batch, `primaryKey, batchId, object, price, qty` in, one row per batch out with `batchId, object_0, price_0, qt_0, …, object_16, price_16, qt_16, lowest_price, highest_price, total_price, mean_price`.

```yaml
group:
  by: [batchId]           # source columns; one output row per batch
  order_by: [line]        # helpers; default: file order
  sorted: true            # rows of a batch are next to each other
  overflow: error         # more rows than slots: error | drop_extra
helpers:
  line:  { source: primaryKey, type: integer }
  price: { source: price, type: decimal }
  qty:   { source: qty, type: integer }
columns:
  batchId:       { source: batchId }
  "object_{n}":  { source: object }
  "price_{n}":   { formula: "[price]" }
  "qt_{n}":      { source: qty }
  lowest_price:  { formula: "lowest([price])" }
  highest_price: { formula: "highest([price])" }
  total_price:   { formula: "total([price] * [qty])" }
  mean_price:    { formula: "mean([price])" }
```

- **Batch key**: `by` lists source columns, compared as text after trimming and null tokens. A rule with a plain `source` is allowed only on a key column.
- **Order**: `order_by` lists helpers, so values sort by type (`10` after `9`). Ascending; ties keep file order.
- **Slot columns**: a rule key containing `{n}` matches the target columns with a number in that place. The first row of a batch fills the lowest number, so `object_1`…`object_17` works like `object_0`…`object_16`. Every pattern must match the same numbers, which give the slot count; a column matched by both a pattern and a plain key is an error. Inside a slot rule, `value` and column references read that slot's row. Unused slots are empty or get the rule's default; an empty slot in a `required` column is a row error.
- **Other columns** reduce the batch: outside slots, a reference to per-row data must sit inside an aggregate. A bare reference is a validation error ("[price] has one value per row of a batch: use first, last, lowest, …"). Nothing is picked silently.
- **Overflow**: `error` makes the batch a row error; `drop_extra` writes the first slots and counts dropped rows in a warning.
- **Sorted input** (`sorted: true`): batches are built while reading and only one is held in memory. The engine remembers the keys it has seen; a key that comes back fails the file: `batch "42" starts again at line 5000 (first seen at line 10): the file is not sorted by batchId`.
- **Unsorted input** (`sorted: false`): every batch is held in memory until the end of the file and written in order of first appearance. Memory grows with the file; Studio warns.
- **What reads what**: helpers are computed for each row of the batch and read only helpers. Aggregates cover every row of the batch (including rows `drop_extra` leaves out of the slots) and read helpers. Columns outside slots read the key, aggregates and other output columns; a default formula cannot use aggregates here.
- **Errors** apply to the output row. With `skip_row`, the batch is not written and the reject file receives every input row of the batch, each with the error.
- **Counts**: the summary reports rows read, batches and rows written (`10 rows read in 4 batches, 3 written`); rows written, rejected and with errors count output rows, and so does `max_error_rate`. `row()` outside a slot is the batch's first input row.
- **Previews** stop after their number of output rows; with unsorted input they read at most 10,000 rows, since a batch may end anywhere.

Studio:

- **Suggestion**: a target with numbered column families (`object_n`, `price_n`, `qt_n`) gets a banner in the editor: "This target repeats 3 columns 17 times. Do several input rows make one output row?" The same entry is in the mapping settings as **Combine rows…**.
- **Combine dialog**: choose the key with stats from the first 20,000 sample rows ("312 batches, 1 to 14 rows each, next to each other"), the patterns, the order and the overflow behaviour. If the sample is not sorted, the dialog says so and sets `sorted: false`. Each pattern's rule starts from the rule of its first column, or a source named like it (`price_{n}` ← `price`). Stop combining gives each column its own copy of its pattern's rule.
- **Editor**: each pattern is one row, `price_{n} × 17`, edited once; helpers are listed under the columns; autocomplete offers helpers first.
- **Preview**: one row per batch.

### 11.7 Engine changes

- Converters split into parse (raw text → typed value) and format (typed value → output text). Columns without operations keep the current direct path; plain mappings must not get slower.
- Pipeline with operations:

```
[first pass: aggregates] → read → [build batches] → workers: columns in dependency order
                                                  → apply: sequence, strategies, counters → write
```

- Workers receive batches of rows, or of grouped batches, as today; the ordered apply stage keeps output, reject file, logs and summary identical for any number of workers.
- Converter size is measured on every release; `internal/expr` must stay small. v1.1 converters: 7.2 MB (6.8 MB before operations, linux/amd64).
- Plain mappings stay on the direct path: within 1.5% of v1.0 speed (BenchmarkRun).

### 11.8 Compatibility

Mappings stay `version: 1`: operations only add keys. A converter embeds its own engine and always understands its mapping; an older CLI reading a mapping with operations fails with an unknown-field error naming the key. v1.0 is released before operations land.

### 11.9 Testing

- Fuzzing of the formula parser: no panic, printing and parsing round-trip.
- Decimal arithmetic checked against `math/big` on random inputs.
- Golden cases per operation, run through the engine, the CLI and a built converter.
- Parallel parity (1 to 8 workers, batches of 1 to 3 rows) extended to sequences, aggregates and combined rows.
- Combining with a batch size of 1 gives the same output as no grouping.

### 11.10 Milestones

All done.

| # | Scope | Done when |
|---|---|---|
| M6 | Typed values, `internal/expr`, `formula`, helpers; Studio formula field | Golden cases pass for transforms and computed columns; plain mappings within 5% of v1.0 speed |
| M7 | `cases` with the Studio builder, `sequence` | Golden cases pass; sequence identical for 1 to 8 workers |
| M8 | Aggregates, first pass, `default_formula`; Studio default modes | A GB file with an aggregate converts in constant memory |
| M9 | Combining rows (sorted and in memory), slots, batch aggregates; Studio combine flow and grouped preview | The batch example of §11.6 is a golden case |

## 12. Ideas for later

- Header matching that ignores case and spacing, as an option.
- Combined rows: expanding an output row in the preview shows the input lines it came from.
- Condition builder: `and` / `or` within a case, without switching to formula text.
- Split rows: the reverse of combining rows. One wide row (a timestamp and 20 sensor columns) becomes several rows (one per sensor reading), reusing the `{n}` patterns.
- Row filter: `keep_rows_when: "[status] <> \"TEST\""` drops rows that are not errors.
- Explain a cell: click an output cell to see each step from the raw value to the written value.
- Saved test cases: pin a sample row with its expected output; `validate` replays them, so editing a mapping cannot silently change results.
- Lookup tables built into the mapping (product code → name), keeping converters offline. The smallest step towards joins.
- `epoch_s` and `epoch_ms` as date formats, simpler than a formula.
- Sequences: prefix and padding (`ORD-000001`), and continuing across runs so daily files sent to the same software never reuse numbers.
- Combining rows: unsorted files beyond memory (batches written to disk), and `overflow: next_row` (extra rows continue on a new output row with the same key).
- Scientific notation as an output option for decimals.
- Month names in other languages than English.
- Check a file against a target: which rows and values do not conform. A target is also a validator, useful to test converted output.
- Mappings remember a fingerprint of their target file, so opening one says what changed since it was last saved.
