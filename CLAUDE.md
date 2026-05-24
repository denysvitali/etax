# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

eTax Converter is a Go CLI tool that converts bank/broker exports into Swiss eCH-0196 style tax statement output. The first supported provider is Interactive Brokers Flex Query XML (`ibkr`).

## Build and Test

```bash
# Build binary
go build -o etax ./cmd/etax

# Run all tests
go test ./...

# Run a single test
go test ./internal/ech0196 -run TestIBKRToECH0196

# Run vet
go vet ./...
```

## CLI Commands

```bash
# Convert IBKR export to eCH-0196 XML
./etax convert --provider ibkr --input ibkr_2025.xml --output tax2025.xml --canton ZH --year 2025 --pdf tax2025.pdf

# Validate XML against XSD schema (requires xmllint)
./etax validate --xml tax2025.xml --schema schemas/eCH-0196-2-2.xsd

# Fetch IBKR Flex Query export
./etax fetch --provider ibkr --token "$IBKR_TOKEN" --query-id 12345 --output ibkr_2025.xml

# Download ESTV Kursliste for ISIN enrichment
./etax kursliste download --year 2025

# Run web server for conversion UI
./etax serve --addr 127.0.0.1:8080

# Decode PDF417 barcode images (for testing)
./etax decode-pdf417 [--zlib] [--output payload.xml] barcode.png [...]
```

## Architecture

### Provider Contract

Providers parse native exports into `internal/domain.Report`. To add a provider, implement `domain.Provider` and register it in `internal/provider/registry.go`.

```go
type Provider interface {
    ID() string
    Name() string
    Parse(context.Context, io.Reader) (*domain.Report, error)
}
```

The `domain.Report` contains `Positions`, `Trades`, and `CashFlows`, all using `money.Decimal` (a `big.Rat` wrapper with XML marshaling support).

### Conversion Pipeline

`cmd/etax/main.go` orchestrates the conversion flow:

1. **Parse** — Provider parses native XML into `domain.Report`
2. **Enrich** — `kursliste` downloads/caches the ESTV ICTax Kursliste and enriches securities by ISIN with Valor metadata
3. **Map** — `ech0196.FromReportWithOptions()` maps the domain report to `ech0196.TaxStatement`
4. **Output** — `ech0196.WriteFile()` writes the XML; optionally `pdf.WriteSummary()` generates a PDF

### eCH-0196 Mapping (`internal/ech0196`)

`mapper.go` is the core mapping logic. Key behaviors:

- Securities are grouped by ISIN from positions, trades, and cash flows
- `buildSecurities()` creates `Security` entries with `TaxValue` (year-end position), `Stocks` (mutations), and `Payments` (dividends/interest/withholding tax)
- `paymentsFor()` matches withholding tax entries to dividend/interest payments by date + normalized description
- Gross revenue is split A (CH ISINs) vs B (foreign ISINs)
- `updateTotals()` rolls up all totals into the statement and `listOfSecurities`
- Document ID is derived from a hash of the institution name plus the client account ID

### PDF Generation (`internal/pdf`, `internal/pdf/pdf417ech`)

The PDF output contains:
1. Human-readable summary pages with a table of securities, positions, trades, and payments
2. Macro PDF417 barcode pages encoding the zlib-compressed XML payload

`internal/pdf/pdf417ech` is a custom PDF417 encoder that fixes eCH geometry at 13 columns, 35 rows, EC level 4, and 290x35 barcode images. It generates Macro PDF417 with segment-count metadata. The barcode package (`internal/barcode`) provides PDF417 decoding for testing using zxinggo.

### Kursliste (`internal/kursliste`)

Downloads and caches the full ESTV ICTax Kursliste XML from `ictax.admin.ch`. `LoadForISINs()` parses the (often large) XML but only stores securities matching the requested ISINs to keep memory reasonable.

### Validation

`ech0196.ValidateXML()` shells out to `xmllint` for XSD validation. The `schemas/` directory contains the official eCH-0196 v2.2.0 XSD and its imported schemas with local import paths so validation works offline.

### HTTP Server (`internal/server`)

A simple multipart form upload server that exposes the same conversion flow through a web UI.
