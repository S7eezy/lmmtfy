# lmmtfy — Let Me Map That For You

Map messy, heterogeneous CSV files onto one fixed format, then apply that mapping from a desktop editor, from the command line, or with a standalone converter binary built from the mapping.

See [DESIGN.md](DESIGN.md) for the full design.

## Status

M1: the engine and the `convert` and `validate` commands work. Next: type detection (`inspect`, `init`), then standalone converters (`build`), then the Studio editor.

```bash
lmmtfy convert input.csv --mapping acme.mapping.yaml -o output.csv [--rejects path] [--report report.json] [--log-level warnings] [--strict]
lmmtfy validate --mapping acme.mapping.yaml [--input input.csv]
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
