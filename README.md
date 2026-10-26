# lmmtfy — Let Me Map That For You

Map messy, heterogeneous CSV files onto one fixed format, then apply that mapping from a desktop editor, from the command line, or with a standalone converter binary built from the mapping.

See [DESIGN.md](DESIGN.md) for the full design.

## Status

M4: the engine, the CLI, standalone converters and the Studio desktop editor work. Next: packaging and hardening (M5).

**lmmtfy Studio** opens a CSV file, detects how to read it, and lets you rename, retype, reorder, delete and add columns, set defaults, optional columns, value maps and error handling, with a live preview of the output. It saves the mapping, converts files and exports standalone converters.

```bash
lmmtfy inspect input.csv                                   # what is this file?
lmmtfy init input.csv -o acme.mapping.yaml                 # draft mapping to review and edit
lmmtfy convert input.csv --mapping acme.mapping.yaml -o output.csv [--rejects path] [--report report.json] [--log-level warnings] [--strict]
lmmtfy validate --mapping acme.mapping.yaml [--input input.csv]
lmmtfy build --mapping acme.mapping.yaml --os windows -o acme-converter   # standalone converter for production
acme-converter.exe input.csv output.csv
```

Example mappings, with their inputs and expected outputs, live in [testdata/](testdata/).

## Development

Requires Go (version in `go.mod`) and `make`.

```bash
make check    # formatting, vet, tests
make build    # converter templates for every platform + CLI for this machine, in dist/
make smoke    # build, then run the CLI and this machine's converter template
make help     # all targets
```

`make build` cross-compiles the converter templates first, then embeds them into the CLI. A plain `go build ./cmd/lmmtfy` also works, but produces a CLI with no embedded templates.

Studio additionally needs Node.js (and the Xcode command line tools on macOS, WebKitGTK on Linux):

```bash
make studio           # studio/build/bin/, for this machine
make studio-windows   # studio/build/bin/lmmtfy-studio.exe, cross-compiled
cd studio && go tool wails dev   # live-reloading development window
```
