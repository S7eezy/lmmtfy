# lmmtfy user guide

lmmtfy (Let Me Map That For You) turns messy CSV files into the exact CSV format a piece of software expects. You describe once, per kind of input file, how its columns become the expected ones. That description, a **mapping**, is then applied by:

- **lmmtfy Studio**, the desktop editor,
- the **`lmmtfy` command line**,
- **standalone converters**: single programs with one mapping built in, for production machines.

All three use the same engine and give byte-for-byte identical results.

Contents: [Concepts](#concepts) · [Installing](#installing) · [Quick start](#quick-start) · [Studio](#studio) · [Mapping reference](#mapping-reference) · [Date formats](#date-formats) · [Commands](#commands) · [Standalone converters](#standalone-converters) · [Running in production](#running-in-production) · [Troubleshooting](#troubleshooting)

## Concepts

| | |
|---|---|
| **Target** | The format the receiving software expects: output columns (names, order, types, constraints) and the output file format (encoding, delimiter, quoting, line endings). It lives inside a mapping, or in its own file shared by several mappings. |
| **Mapping** | How one kind of input file becomes the target: how to read the input, which source column fills each target column, how values are converted, what happens on errors. One mapping per source format: when a source sends a different layout, make another mapping. |
| **Converter** | A standalone program with one mapping built in, created by `lmmtfy build` or Studio. It needs nothing else to run. |

## Installing

Everything is portable: unpack the archive anywhere (your home folder, a USB stick, a network share) and run. Nothing is installed, no admin rights are needed. Each archive has a `README.txt` with platform notes.

| Archive | Contents |
|---|---|
| `lmmtfy-<version>-<os>-<arch>` | The command line tool, this guide, an example. Converter templates for every platform are built in. |
| `lmmtfy-studio-<version>-windows-amd64.zip` | Studio for Windows: a single `lmmtfy-studio.exe`. |
| `lmmtfy-studio-<version>-macos.zip` | Studio for macOS (Apple Silicon and Intel). |

`SHA256SUMS` lists a checksum per archive. Check copies made to other machines with `sha256sum -c SHA256SUMS` (Linux), `shasum -a 256 -c SHA256SUMS` (macOS) or `Get-FileHash <file>` (Windows PowerShell).

## Quick start

With the command line, from a CSV file to a production converter:

```bash
lmmtfy inspect orders.csv                         # what is this file?
lmmtfy init orders.csv -o orders.mapping.yaml     # draft mapping, to review
# edit orders.mapping.yaml: names, types, target output format
lmmtfy validate --mapping orders.mapping.yaml --input orders.csv
lmmtfy convert orders.csv --mapping orders.mapping.yaml -o converted.csv
lmmtfy build --mapping orders.mapping.yaml --os windows -o orders-converter
```

`init` writes comments in the mapping: what was detected in each column, and `REVIEW:` lines for what a human should check (ambiguous day/month order, separators, values that did not fit a type).

The archive contains a complete example in `examples/acme-orders/`: a Latin-1, semicolon-separated export with title lines, a total line, decimal commas, mixed date formats and status codes.

```bash
lmmtfy convert examples/acme-orders/input.csv --mapping examples/acme-orders/mapping.yaml -o out.csv
```

## Studio

The home screen has two sides: **mappings** and **targets**, each with its recent files. Dropping a file on the window opens it: a CSV file starts a new mapping, a `*.target.yaml` file opens the target editor, other YAML files open as mappings.

### A new mapping

1. **New mapping from a CSV file**. Studio detects how to read it.
2. **Read the file.** Check encoding, delimiter, quotes, header line, lines to skip at the top and bottom, values to read as empty. The preview marks each row: skipped (struck through), header (bold), data, footer. It updates as you change settings.
3. **Target**: what the file becomes. Three choices:
   - **Use a target file**: the format is fixed, the file is mapped onto it. Columns with matching names are mapped right away; the card shows how many target columns were found and which ones are left, required ones in red.
   - **New target file** (the default): the target starts as this file's columns, and is saved as its own file next to the mapping (`<name>.target.yaml`) on the first save, so the next source can reuse it.
   - **Inside this mapping**: for a one-off mapping nobody will share.
   You can switch between them freely; the target drafted from the file is kept.
4. **Settings**: mapping name, what happens with values that cannot be converted, logging.
5. **Map columns.** Each row is an output column, in output order: the source column it comes from (with its detected type and confidence), then the output name and type.
   - With a new or inside target, everything is editable: rename, retype, drag rows to reorder, delete, add columns; unused source columns are listed below the table, click one to add it.
   - With a target file, the output columns are the file's: names, types and order are shown but locked. You choose where each value comes from and how it is read. To change the target itself, use **Edit target…** (see below).
   - Select a column to see what was detected and to set its options on the right: source, empty and missing values, how values are read, value map, default, error handling, and how the value is written (shown read-only for a target file).
   - The **Review** tab lists what needs a decision: target columns with no source, source columns missing from the file, columns matched to a target by name (confirm them, one by one or all at once), types that do not match the data, rules for a column the target no longer has, unused source columns. Click an item to jump to its column.
   - The other tabs preview the converted first 100 rows, rejected rows and messages. Problems with the mapping mark their column; saving and converting wait until they are fixed.
6. **Save** writes the mapping (YAML), and a new target file with it. **Convert…** converts a whole file. **Export converter…** builds converters for Windows, Linux and macOS.

Opening a saved mapping goes straight to the column editor; open a sample CSV file to get the preview. Every step stays reachable from the steps at the top, the Target step included: there you can link another target file, copy the current one into a new file, or keep the target inside the mapping.

### Targets

A target file is shared by every mapping that uses it, so Studio only changes it in the **target editor**, never by saving a mapping.

- **New target**: **from an example of the expected file** (a file the receiving software exports or documents, not a partner's file) or **blank**. From an example, Studio drafts the columns with their exact header names, types and formats, and the file format: delimiter, encoding, line endings, date formats, decimal and thousands separators.
- **Open a target** to change its columns (name, type, required, default, constraints, output format; drag to reorder) or, in **Settings**, its name and file format.
- **New mapping for this target** starts a new mapping with the target already chosen.
- **Edit target…** in a mapping opens the editor on its target file. Back in the mapping, the target is reloaded: renamed columns are renamed in the mapping too, and the Review list shows what changed.

Mappings refer to target columns by name. Renaming or removing a column leaves the rule of every other mapping using the target behind: their Review list shows it, with a button to remove the rule, and asks to map the new column.

## Mapping reference

A mapping is a YAML file. Studio and `init` write complete ones; this section explains every option. Options left out take the default shown in **bold**.

```yaml
version: 1
name: acme-orders
description: ACME weekly export
target: erp-orders.target.yaml      # or the target written inline, see below
input:    { ... }                   # how to read source files
on_error: { ... }                   # what to do with bad values
logging:  { ... }
columns:                            # one rule per target column
  order_id:   { source: "N° Commande" }
  amount:     { source: "Montant TTC", decimal_sep: ",", thousands_sep: " " }
  status:     { source: "Etat", map: { V: VALIDATED, A: CANCELLED } }
  country:    { default: FR }
```

### Target

```yaml
version: 1
name: erp-orders
output:
  encoding: utf-8
  delimiter: ","
  line_ending: crlf
columns:                            # output order
  - { name: order_id, type: integer, required: true }
  - { name: order_date, type: date, format: "YYYY-MM-DD" }
  - { name: amount, type: decimal, scale: 2 }
```

A mapping refers to a target file with a path relative to the mapping file, or holds the same content inline under `target:`.

**Output format** (`target.output`):

| Option | Values | Default |
|---|---|---|
| `encoding` | `utf-8`, `utf-16le`, `utf-16be`, `windows-1252`, `latin-1`, `iso-8859-15`, any IANA name | **utf-8** |
| `bom` | write a byte order mark | **false** |
| `delimiter` | one character, or `comma`, `semicolon`, `tab`, `pipe`, `space` | **`,`** |
| `quote` | one character, or `none` | **`"`** |
| `quote_style` | `minimal` (when needed), `always`, `non_numeric`, `never` | **minimal** |
| `escape` | `double` (`""`), `backslash` (`\"`) | **double** |
| `line_ending` | `crlf`, `lf` | **crlf** |
| `header` | write a header row | **true** |
| `null_value` | how empty values are written | **empty** |

**Target columns**: `name`, `type`, `required` (**false**: when true an empty value is an error), `default` (used when the mapping rule has none), plus type options:

| Type | Output options |
|---|---|
| `string` | `case` (`upper`, `lower`), `max_length`, `on_overflow` (**`error`**, `truncate`), `pattern` (regular expression matching the whole value), `allowed_values` |
| `integer` | `min`, `max`, `thousands_sep` |
| `decimal` | `scale` (decimals written; none: as many as the value has), `rounding` (**`half_up`**, `half_even`, `truncate`), `decimal_sep` (**`.`**), `thousands_sep`, `min`, `max`. Source values may use scientific notation (`5.4e-07`, `1E+3`); output is always plain notation (`0.00000054`). |
| `boolean` | `true_value` (**`true`**), `false_value` (**`false`**) |
| `date`, `datetime`, `time` | `format` (**`YYYY-MM-DD`**, **`YYYY-MM-DDTHH:mm:ss`**, **`HH:mm:ss`**), `timezone` (datetime only, **UTC**) |

### Input

| Option | Meaning | Default |
|---|---|---|
| `encoding` | as for the output | **utf-8** |
| `bom` | `auto`/`true`: remove a byte order mark if present; `false`: keep it | **auto** |
| `delimiter`, `quote` | as for the output | **`,`**, **`"`** |
| `escape` | `double`, `backslash`, `none` | **double** |
| `header` | the file has a header row | **true** |
| `header_row` | line of the header (or of the first data row without header); lines above are skipped | **1** |
| `skip_rows_after_header` | rows ignored right after the header | **0** |
| `skip_footer` | rows ignored at the end (totals…) | **0** |
| `comment_prefix` | lines starting with it are ignored | none |
| `null_tokens` | values read as empty, e.g. `[NULL, N/A, "-"]` | none |
| `trim` | remove spaces around values | **false** |
| `ragged_rows` | rows with a different field count: `error`, `pad` (short rows filled with empty values), `truncate` (extra fields ignored), `lenient` (both) | **error** |
| `duplicate_headers` | `error`, or `suffix`: Name, Name_2, Name_3 | **error** |

Header names are trimmed before matching rules.

### Column rules

| Option | Meaning | Default |
|---|---|---|
| `source` | header name of the source column | |
| `source_index` | position (1 = first) instead, for files without header | |
| `default` | value used when needed; written in the **output** format (`"0,00"` if the target writes decimal commas). `""` means empty | |
| `optional` | the source column may be missing from a file | **false** |
| `if_missing` | when the source column is missing: `error`, `empty`, `default`, `omit` (column removed from the output) | **error**, **empty** if optional |
| `if_empty` | when a value is empty: `keep`, `default`, `error` | **keep** |
| `map` | value map `{ source value: output value }`, applied before conversion | |
| `map_unmatched` | values not in the map: `keep`, `error`, `default` | **keep** |
| `on_error` | error strategy for this column only | the global one |
| `trim` | override `input.trim` for this column | |
| `formats` | date/time formats of the source, tried in order | the output format |
| `decimal_sep`, `thousands_sep` | separators of the source | **`.`**, none |
| `true_values`, `false_values` | source booleans, case-insensitive | **true t yes y 1** / **false f no n 0** |
| `timezone` | time zone of source datetimes without offset, e.g. `Europe/Paris` | the target's |

A target column without rule is left empty (or gets the target `default`); a required one must have a rule. A source column without rule is dropped.

Each value goes through: trim → null tokens → value map → empty handling → conversion to the type → checks (min, max, pattern…) → output formatting.

**Optional columns.** When optional source columns are missing, a warning lists them at the start of the conversion:

```
WARN  configuration has 2 optional columns missing from input: Etat, Pays
      Etat → status: default "PENDING"
      Pays → country: left empty
```

`if_missing: omit` changes the set of output columns, which the receiving software may refuse: Studio and `validate` warn about it.

### Errors

```yaml
on_error:
  strategy: skip_row        # fail_file | skip_row | default
  max_error_rate: 0.01      # fail when more than 1% of rows have errors
  max_errors: 1000          # fail as soon as more than 1000 rows have errors
  reject_file: true
```

| Strategy | Effect |
|---|---|
| **`fail_file`** | The first bad value stops the conversion. No output file is written. |
| `skip_row` | The row is left out of the output and written to the reject file. |
| `default` | The bad value is replaced by the column default (empty when there is none). |

The **reject file** (`output.rejects.csv` next to `output.csv`, **enabled** by default) holds the rejected rows as they were read, in the input format, plus `_line`, `_column` and `_error` columns: fix them and convert them again with the same mapping. No reject file is written, and an old one is removed, when no row is rejected.

A conversion never leaves a partial file: output is written to a temporary file and renamed at the end, only on success.

### Logging

```yaml
logging:
  level: errors      # none | errors | warnings | info | trace (every row)
  format: text       # text | json (one object per line)
  file: ""           # empty: standard error
  summary: true      # summary at the end
```

### YAML tips

- Values are read as written: `01` stays `01`, `1.50` stays `1.50`, `yes` stays `yes`.
- Quote values starting with `-`, `*`, `|`, `>`, `!`, `&`, `%`, `@` or containing `: ` or ` #`, e.g. `default: "-"`.
- Write a tab as `"\t"` or `tab`.
- A typo in an option name is an error, with its line and column: options are never silently ignored.

## Date formats

| Token | Meaning | Token | Meaning |
|---|---|---|---|
| `YYYY` `YY` | year | `HH` | hour 00–23 |
| `MM` | month 01–12 | `hh` `h` | hour 01–12, with `A`/`a` for AM/PM |
| `M` | month, 1 or 2 digits | `mm` `m` | minutes |
| `MMM` `MMMM` | `Jan` / `January` (English) | `ss` `s` | seconds |
| `DD` | day 01–31 | `.SSS` `.SSSSSS` | fractional seconds, after `.` or `,` |
| `D` | day, 1 or 2 digits | `Z` / `ZZ` | offset `+02:00` (or `Z`) / `+0200` |

Spaces, punctuation and `T` are written as is. Examples: `DD/MM/YYYY`, `YYYYMMDD`, `D MMM YYYY`, `YYYY-MM-DDTHH:mm:ssZ`.

## Commands

```
lmmtfy convert <input.csv> --mapping <m.yaml> -o <output.csv> [options]
    --rejects <path>      reject file (default: <output>.rejects.csv when enabled)
    --report <r.json>     write a JSON summary
    --log-level <level>   override the mapping's log level
    --strict              exit with code 1 when any row had an error, even if handled
    --workers <n>         CPU cores used (default: all)
lmmtfy validate --mapping <m.yaml> [--input <input.csv>]
lmmtfy inspect <input.csv> [--sample <rows>] [--json]
lmmtfy init <input.csv> -o <m.yaml> [--name <name>] [--sample <rows>] [--force]
lmmtfy build --mapping <m.yaml> --os windows|linux|darwin [--arch amd64|arm64] -o <converter>
lmmtfy version
```

Options may come before or after file names.

**Exit codes** (converters use the same):

| Code | Meaning |
|---|---|
| 0 | Success. Handled errors (rejected or defaulted rows) are allowed, unless `--strict`. |
| 1 | Success with handled errors, only with `--strict`. |
| 2 | Bad command line or invalid mapping. |
| 3 | The file was refused: `fail_file`, error thresholds, required source column missing, malformed CSV. |
| 4 | A file could not be read or written. |
| 130 | Interrupted (Ctrl+C or stop signal). No output written. |

## Standalone converters

```bash
lmmtfy build --mapping orders.mapping.yaml --os windows -o orders-converter
# → orders-converter.exe
```

Platforms: `windows` (amd64), `linux` (amd64, arm64), `darwin` (arm64, amd64). Any platform's converter can be built on any machine, offline.

```
orders-converter <input.csv> <output.csv> [--rejects path] [--report r.json] [--log-level L] [--strict] [--workers N]
orders-converter --info           # mapping name and checksum, engine version, build date
orders-converter --print-config   # the embedded mapping
```

A converter cannot be pointed at another mapping: it converts with the one it was built with, always. Its checksum (`--info`) matches the `mapping_sha256` of `--report` summaries, so a converted file can be traced back to the exact configuration. A damaged converter refuses to run.

## Running in production

- Copy the converter (and nothing else) to the machine. Check it with `SHA256SUMS` or `--info`.
- Schedule it with the usual tools (Windows Task Scheduler, cron, systemd timers) and act on the exit code: `0` done, `3` file refused (look at the log and the reject file), `4` file access problem.
- Keep `--report summary.json` for each run: rows read, written and rejected, errors per column, missing optional columns, mapping checksum.
- Set `logging.file` in the mapping, or redirect standard error, to keep logs.
- Large files are streamed: memory stays flat whatever the size (about 40 MB with all cores for a 1 GB file).

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| Accented letters come out wrong (`Ã©`, `�`) | Wrong input encoding. Exports from Windows tools are usually `windows-1252`. |
| Everything lands in one column | Wrong delimiter. `lmmtfy inspect` shows the detected one. |
| `required source columns missing from input: "Date" (for order_date)` | The file has no such header, or it is spelled differently, or the header is not on `header_row`. `lmmtfy validate --mapping m.yaml --input file.csv` shows how each column is found. |
| `duplicate header "Name"` | Set `input.duplicate_headers: suffix` and use `Name_2` for the second one. |
| `row has 4 fields, expected 5` | Rows with missing or extra fields: fix the file, or set `input.ragged_rows`. |
| `"31/02/2026" does not match date format DD/MM/YYYY` | The value is invalid (there is no 31 February) or another format is used: add it to `formats`. |
| Decimal values rejected | Set `decimal_sep` and `thousands_sep` on the rule to the source's separators. Scientific notation (`5.4e-07`) is accepted. |
| Tiny values become `0.00` | The column has a `scale` too small for them (often with scientific notation). Remove the scale to keep all digits, or raise it. |
| The total line at the end is rejected | Set `input.skip_footer`. |
| Windows: "Windows protected your PC" | SmartScreen warns about unsigned programs: "More info", then "Run anyway". Or sign the files with your organization's certificate (sign converters after building them). |
| macOS: "cannot verify the developer" | Files downloaded with a browser are quarantined: right-click, Open, then Open. Copies from a network share or USB stick are not affected. |
| Studio on Windows does not start or shows a blank window | The WebView2 runtime is missing: install it, or unpack Microsoft's Fixed Version WebView2 runtime into a `webview2` folder next to `lmmtfy-studio.exe`. |
