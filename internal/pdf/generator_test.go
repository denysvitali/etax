package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"

	pdf417decoder "etax/internal/barcode"
	"etax/internal/ech0196"
	"etax/internal/pdf/pdf417ech"
)

func TestBytesCreatesPDFWithBarcodeSegments(t *testing.T) {
	stmt := ech0196.NewTaxStatement("doc-1", "ZH", 2025)
	stmt.Institution = ech0196.Institution{Name: "Interactive Brokers", LEI: "5493004J90J71E0E4R31"}
	xmlData := []byte(`<taxStatement id="doc-1"></taxStatement>`)

	data, err := Bytes(stmt, xmlData)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("expected PDF header, got %q", data[:min(8, len(data))])
	}
	segments, err := Segments(stmt.ID, xmlData)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) == 0 || segments[0].DocumentID != "doc-1" {
		t.Fatalf("unexpected segments: %#v", segments)
	}
	if segments[0].MacroFileID == "" || segments[0].MacroFileID == segments[0].DocumentID {
		t.Fatalf("unexpected macro file id: %#v", segments[0])
	}
	if segments[0].Index != 0 || !segments[len(segments)-1].Last {
		t.Fatalf("unexpected macro segment metadata: %#v", segments)
	}
	zr, err := zlib.NewReader(bytes.NewReader(segments[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	roundTrip, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(roundTrip) != string(xmlData) {
		t.Fatalf("compressed XML round trip = %q, want %q", roundTrip, xmlData)
	}
	code, err := pdf417ech.EncodeMacroSegment(pdf417ech.MacroSegment{
		FileID:       segments[0].MacroFileID,
		FileName:     segments[0].DocumentID,
		SegmentIndex: segments[0].Index,
		SegmentCount: segments[0].Total,
		Data:         segments[0].Data,
		Last:         segments[0].Last,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code.Bounds().Dx() != 290 || code.Bounds().Dy() != 35 {
		t.Fatalf("barcode dimensions = %dx%d, want 290x35", code.Bounds().Dx(), code.Bounds().Dy())
	}
}

func TestPageMarkerContentUsesECH0196NumericLayout(t *testing.T) {
	stmt := ech0196.NewTaxStatement("doc-1", "ZH", 2025)

	if got := pageMarkerContent(stmt, 7, false); got != "1962200000007002" {
		t.Fatalf("non-barcode page marker = %s", got)
	}
	if got := pageMarkerContent(stmt, 7, true); got != "1962200000007102" {
		t.Fatalf("barcode page marker = %s", got)
	}
}

func TestScaledPDF417ImageRemainsDecodable(t *testing.T) {
	code, err := pdf417ech.EncodeMacroSegment(pdf417ech.MacroSegment{
		FileID:       "123456234111",
		FileName:     "CH00000000000U12345672025123101",
		SegmentIndex: 0,
		SegmentCount: 1,
		Data:         []byte("payload"),
		Last:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	img, err := pngBytes(scaleNearest(code, pdf417ImageWidth, pdf417ImageHeight))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pdf417decoder.DecodePDF417Bytes(bytes.NewReader(img)); err != nil {
		t.Fatalf("scaled PDF417 image is not decodable: %v", err)
	}
}
