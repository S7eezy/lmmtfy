<p align="center">
  <img src="docs/logo.png" alt="lmmtfy logo" width="112" height="112">
</p>

<h1 align="center">lmmtfy</h1>

<p align="center">
  <b>Let Me Map That For You.</b><br>
  Messy CSV in. The exact format your software expects out.
</p>

<p align="center">
  <a href="https://github.com/S7eezy/lmmtfy/actions/workflows/ci.yml"><img src="https://github.com/S7eezy/lmmtfy/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white" alt="Go 1.27">
  <img src="https://img.shields.io/badge/runs_on-Windows%20%7C%20Linux%20%7C%20macOS-555" alt="Windows, Linux, macOS">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT license"></a>
</p>

---

Your software imports one strict CSV format. Your partners send something else: Latin-1 exports with title lines, semicolons, decimal commas, three date formats in one column, status codes nobody documented, a total line at the bottom.

lmmtfy lets you describe **once** how each of those sources maps onto your format. Then any file from that source converts in one command, in a desktop app, or with a small standalone program you drop on a production server.

## What it does

This file:

```
Export ACME;;;;
Généré le 01/10/2026;;;;
N° Commande;Date;Montant TTC;Etat;Remarque
1001;05/10/2026;1 234,50;V;Livraison rapide
1002;06-10-2026; 99,9 ;A;
Total;3;;;
```

with this mapping:

```yaml
input:   { encoding: latin-1, delimiter: ";", header_row: 3, skip_footer: 1, trim: true }
columns:
  order_id:   { source: "N° Commande" }
  order_date: { source: "Date", formats: ["DD/MM/YYYY", "DD-MM-YYYY"] }
  amount:     { source: "Montant TTC", decimal_sep: ",", thousands_sep: " " }
  status:     { source: "Etat", map: { V: VALIDATED, A: CANCELLED } }
  country:    { default: FR }
  comment:    { source: "Remarque", if_empty: default, default: "-" }
```

becomes:

```
order_id,order_date,amount,status,country,comment
1001,2026-10-05,1234.50,VALIDATED,FR,Livraison rapide
1002,2026-10-06,99.90,CANCELLED,FR,-
```

You rarely write that mapping by hand: `lmmtfy init` or the Studio detects the encoding, delimiter, header line, footer and column types, and drafts it for you.

## Three ways to run a mapping

| | |
|---|---|
| **Studio** | Desktop editor. Open a CSV, check how it is read, map its columns with a live preview of the output, save, convert, export converters. |
| **CLI** | `lmmtfy convert input.csv --mapping acme.yaml -o output.csv` for scripts and schedulers. |
| **Converter** | `lmmtfy build --mapping acme.yaml --os windows -o acme-converter` produces `acme-converter.exe`: one file, mapping built in, nothing to install. Built for Windows, Linux or macOS, from any of them, offline. |

All three run the same engine and produce byte-for-byte identical files.

## Highlights

- **Detection you can trust.** Each column's type comes with how many sampled values fit it, colour-coded in Studio. Columns that are *almost* dates or numbers are flagged, with the values that do not fit.
- **Built for wide files.** Rename, retype, drop, add columns with defaults, reorder by drag and drop. Comfortable with 250 columns.
- **Strict by default.** A bad value fails the file, and nothing is written. Or reject bad rows to a reject file, or fall back to defaults; set error thresholds. Optional columns, value maps, required values, patterns, allowed values.
- **Fast.** Files are streamed and converted on every core: a 1 GB file takes about 7 seconds on a laptop, in about 40 MB of memory.
- **Made for locked-down production.** Portable archives, no admin rights, no network. A converter cannot be pointed at another mapping, refuses to run if damaged, and reports the checksum of the configuration it embeds.

## Getting started

Download a portable archive from [Releases](https://github.com/S7eezy/lmmtfy/releases): unpack anywhere and run. Or build it, see [Building](#building).

```bash
lmmtfy inspect orders.csv                        # what is in this file?
lmmtfy init orders.csv -o orders.mapping.yaml    # draft a mapping, then review it
lmmtfy convert orders.csv --mapping orders.mapping.yaml -o clean.csv
lmmtfy build --mapping orders.mapping.yaml --os linux -o orders-converter
```

Studio does the same with a window: **New mapping from a CSV file**, three steps, done.

The **[user guide](docs/USER_GUIDE.md)** covers every option, the date format tokens, exit codes, running converters in production, and troubleshooting.

## Building

Needs Go 1.27 and `make`. Studio also needs Node.js, plus the Xcode command line tools on macOS or WebKitGTK 4.1 on Linux.

```bash
make check     # formatting, vet, tests
make build     # CLI for this machine in dist/, with converter templates for every platform
make studio    # Studio for this machine in studio/build/bin/
make dist      # portable release archives in dist/release/
```

`cd studio && go tool wails dev` runs Studio with live reload.

## How it works

- One Go engine, shared by the CLI, Studio and converters. CSV is read with a streaming tokenizer, rows are converted in parallel and written back in order.
- Every behaviour is pinned by golden tests: real inputs, expected outputs, reject files, reports and logs, checked byte for byte through the CLI and through real converter programs.
- A converter is a prebuilt template with a 1 MiB slot that `lmmtfy build` fills with the mapping. On macOS the code signature is recomputed afterwards, so even a Linux machine can produce a working Mac converter.

The full design is in **[DESIGN.md](DESIGN.md)**.

## Built with

[Go](https://go.dev), [Wails](https://wails.io), [Svelte](https://svelte.dev), [Lucide](https://lucide.dev) icons, [goccy/go-yaml](https://github.com/goccy/go-yaml), [shopspring/decimal](https://github.com/shopspring/decimal).

## License

[MIT](LICENSE)
