# eTax Converter

Generic Go tool for converting bank/broker exports into Swiss eCH-0196 style tax statement output.

The provider layer is intentionally small: providers parse their native export into `internal/domain.Report`, while the eCH mapper and output layers stay provider-neutral. The first provider is Interactive Brokers Flex Query XML (`ibkr`).

## Build

```bash
go build -o etax ./cmd/etax
```

The repository includes the official eCH-0196 v2.2.0 XSD and its imported eCH schemas under `schemas/`, with local import paths so validation works offline.

## CLI

```bash
./etax convert --provider ibkr --input ibkr_2025.xml --output tax2025.xml --canton ZH --year 2025 --pdf tax2025.pdf
./etax validate --xml tax2025.xml --schema schemas/eCH-0196-2-2.xsd
./etax fetch --provider ibkr --token "$IBKR_TOKEN" --query-id 12345 --output ibkr_2025.xml
./etax kursliste download --year 2025
./etax serve --addr 127.0.0.1:8080
```

`convert` automatically downloads and caches the latest full ESTV ICTax Kursliste for the selected tax year, then enriches securities by ISIN with official Valor metadata. Use `--auto-kursliste=false` to disable this, `--kursliste-dir` to choose the cache directory, or `--kursliste` to point at a local XML file/directory explicitly.

## Provider Contract

Add a provider by implementing:

```go
type Provider interface {
    ID() string
    Name() string
    Parse(context.Context, io.Reader) (*domain.Report, error)
}
```

Register it in `internal/provider/registry.go`.

## Current PDF Status

The `--pdf` output uses Macro PDF417 barcode pages generated from a zlib-compressed, segmented XML payload, plus a human-readable summary page. Barcode generation is owned by `internal/pdf/pdf417ech`, which fixes the eCH geometry at 13 columns, 35 rows, EC level 4, and 290x35 barcode images.

The remaining production risk is external interoperability validation: the encoder emits Macro PDF417 control blocks and segment-count metadata, but this should still be checked with a scanner/decoder that exposes Macro PDF417 metadata and with target cantonal import software.
