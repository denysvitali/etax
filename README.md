# eTax Converter

Generic Go tool for converting bank/broker exports into Swiss eCH-0196 style tax statement output.

The provider layer is intentionally small: providers parse their native export into `internal/domain.Report`, while the eCH mapper and output layers stay provider-neutral. The first provider is Interactive Brokers Flex Query XML (`ibkr`).

## Build

```bash
go build -o etax ./cmd/etax
```

## CLI

```bash
./etax convert --provider ibkr --input ibkr_2025.xml --output tax2025.xml --canton ZH --year 2025 --pdf tax2025.pdf
./etax validate --xml tax2025.xml --schema schemas/eCH-0196-2-2.xsd
./etax fetch --provider ibkr --token "$IBKR_TOKEN" --query-id 12345 --output ibkr_2025.xml
./etax serve --addr 127.0.0.1:8080
```

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

The `--pdf` output is a lightweight PDF summary generated without external dependencies. The strict import artifact is the eCH XML. Production-grade eCH-0270 PDF417 structured append and PDF/A XML attachment support should be added behind `internal/pdf` without changing providers.
