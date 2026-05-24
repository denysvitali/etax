package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"sort"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/phpdave11/gofpdf"

	"etax/internal/ech0196"
	"etax/internal/pdf/pdf417ech"
)

const (
	barcodeSegmentsPerPage = 6
	pdf417WidthMM          = 121.8
	pdf417HeightMM         = 28.0
	barcodePixelWidth      = 290
	barcodePixelHeight     = 35
	pdf417ImageWidth       = 2877
	pdf417ImageHeight      = 661
	pageMarkerWidthMM      = 10.0
	pageMarkerHeightMM     = 44.0
)

type Segment struct {
	DocumentID  string
	MacroFileID string
	Index       int
	Total       int
	Data        []byte
	Last        bool
}

type reportRow struct {
	Date      string
	Label     string
	ISIN      string
	Quantity  string
	Currency  string
	UnitPrice string
	FXRate    string
	TaxValue  string
	GrossA    string
	GrossB    string
	WHT       string
	USA       string
	Bold      bool
	Shade     bool
}

func WriteSummary(path string, stmt ech0196.TaxStatement, xmlData []byte) error {
	data, err := Bytes(stmt, xmlData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Bytes(stmt ech0196.TaxStatement, xmlData []byte) ([]byte, error) {
	segments, err := Segments(stmt.ID, xmlData)
	if err != nil {
		return nil, err
	}

	doc := gofpdf.New("L", "mm", "A4", "")
	doc.SetTitle("eTax report "+stmt.ID, false)
	doc.SetAuthor(stmt.Institution.Name, false)
	doc.SetCreator("etax", false)
	barcodePages := (len(segments) + barcodeSegmentsPerPage - 1) / barcodeSegmentsPerPage
	reportPages := countReportPages(stmt)
	totalPages := reportPages + barcodePages
	page := 0

	if err := addReportPages(doc, stmt, totalPages, &page); err != nil {
		return nil, err
	}
	for i := 0; i < len(segments); i += barcodeSegmentsPerPage {
		doc.AddPage()
		page++
		addBarcodeSheetHeader(doc, stmt)
		if err := addPageMarker(doc, stmt, page, true, 12, 14); err != nil {
			return nil, err
		}
		pageSegments := segments[i:min(i+barcodeSegmentsPerPage, len(segments))]
		for j, segment := range pageSegments {
			col := j / 3
			row := j % 3
			x := 34.0 + float64(col)*128.0
			y := 76.0 + float64(row)*36.0
			if err := addBarcodeSegment(doc, segment, x, y); err != nil {
				return nil, err
			}
		}
		addBarcodeSheetFooter(doc, stmt, page, totalPages)
	}

	var out bytes.Buffer
	if err := doc.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func Segments(documentID string, xmlData []byte) ([]Segment, error) {
	compressed, err := compressXML(xmlData)
	if err != nil {
		return nil, err
	}
	fileName := documentID
	fileID := macroFileID(documentID)
	total := 1
	for {
		next := countSegments(fileID, fileName, compressed, total)
		if next == total {
			break
		}
		total = next
	}

	segments := make([]Segment, 0, total)
	for offset, i := 0, 0; offset < len(compressed) || (len(compressed) == 0 && i == 0); i++ {
		remaining := len(compressed) - offset
		lastCap := pdf417ech.MaxSegmentBytes(fileID, fileName, i, total, true)
		last := remaining <= lastCap
		capacity := lastCap
		if !last {
			capacity = pdf417ech.MaxSegmentBytes(fileID, fileName, i, total, false)
		}
		end := min(offset+capacity, len(compressed))
		segments = append(segments, Segment{
			DocumentID:  documentID,
			MacroFileID: fileID,
			Index:       i,
			Total:       total,
			Data:        compressed[offset:end],
			Last:        last,
		})
		offset = end
		if last {
			break
		}
	}
	return segments, nil
}

func addReportPages(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement, totalPages int, page *int) error {
	(*page)++
	doc.AddPage()
	addReportHeader(doc, stmt, "Zusammenfassung", *page, totalPages)
	addSummary(doc, stmt)

	sections := []struct {
		title      string
		rows       []reportRow
		foreignTax bool
	}{
		{"A-values with Swiss withholding tax", rowsForSecurities(stmt, func(sec ech0196.Security) bool {
			return sec.Country == "CH"
		}, false), false},
		{"B-values without Swiss withholding tax", rowsForSecurities(stmt, func(sec ech0196.Security) bool {
			return sec.Country != "CH"
		}, false), false},
		{"Values with foreign withholding tax credit / additional USA tax retention", rowsForSecurities(stmt, func(sec ech0196.Security) bool {
			for _, p := range sec.Payments {
				if !p.WithHoldingTaxClaim.IsZero() || !p.AdditionalWithHoldingUSA.IsZero() {
					return true
				}
			}
			return false
		}, true), true},
	}
	for _, section := range sections {
		if len(section.rows) == 0 {
			continue
		}
		if err := addTablePages(doc, stmt, section.title, section.rows, section.foreignTax, totalPages, page); err != nil {
			return err
		}
	}
	return nil
}

func countReportPages(stmt ech0196.TaxStatement) int {
	pages := 1
	for _, rows := range [][]reportRow{
		rowsForSecurities(stmt, func(sec ech0196.Security) bool { return sec.Country == "CH" }, false),
		rowsForSecurities(stmt, func(sec ech0196.Security) bool { return sec.Country != "CH" }, false),
		rowsForSecurities(stmt, func(sec ech0196.Security) bool {
			for _, p := range sec.Payments {
				if !p.WithHoldingTaxClaim.IsZero() || !p.AdditionalWithHoldingUSA.IsZero() {
					return true
				}
			}
			return false
		}, true),
	} {
		if len(rows) > 0 {
			pages += (len(rows) + reportRowsPerPage() - 1) / reportRowsPerPage()
		}
	}
	return pages
}

func addTablePages(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement, title string, rows []reportRow, foreignTax bool, totalPages int, page *int) error {
	for start := 0; start < len(rows); start += reportRowsPerPage() {
		(*page)++
		doc.AddPage()
		addReportHeader(doc, stmt, title, *page, totalPages)
		addSecurityTableHeader(doc, foreignTax)
		end := min(start+reportRowsPerPage(), len(rows))
		y := 69.0
		for _, row := range rows[start:end] {
			addSecurityRow(doc, y, row, foreignTax)
			y += 4.8
		}
	}
	return nil
}

func addReportHeader(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement, title string, page, totalPages int) {
	doc.SetFont("Helvetica", "B", 16)
	doc.Text(34, 36, fmt.Sprintf("Tax statement in CHF 31.12.%d", stmt.TaxPeriod))
	doc.SetFont("Helvetica", "B", 14)
	doc.Text(34, 45, title)
	addHeaderMeta(doc, stmt, 210, 20)
	_ = addPageMarker(doc, stmt, page, false, 12, 14)
	addBarcodeSheetFooter(doc, stmt, page, totalPages)
}

func addHeaderMeta(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement, x, y float64) {
	doc.SetFont("Helvetica", "", 7)
	doc.Text(x, y, "Customer")
	doc.Text(x+14, y, clientName(stmt))
	doc.Text(x, y+5, "Client no.")
	doc.Text(x+14, y+5, clientNumber(stmt))
	doc.Text(x, y+10, "Period")
	doc.SetFont("Helvetica", "B", 7)
	doc.Text(x+14, y+10, fmt.Sprintf("01.01.%04d - 31.12.%04d", stmt.TaxPeriod, stmt.TaxPeriod))
	doc.SetFont("Helvetica", "", 7)
	doc.Text(x, y+15, "Created")
	doc.Text(x+14, y+15, creationDate(stmt.CreationDate))
	doc.Text(x, y+20, "Canton")
	doc.Text(x+14, y+20, stmt.Canton)
}

func addSummary(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement) {
	doc.SetFillColor(238, 238, 238)
	doc.Rect(34, 55, 180, 58, "F")
	addSummaryCell(doc, 48, 64, "Tax value of", "A- and B-values", stmt.TotalTaxValue.String())
	addSummaryCell(doc, 83, 64, "Gross income", "A-values", stmt.TotalGrossRevenueA.String())
	addSummaryCell(doc, 118, 64, "Gross income", "B-values", stmt.TotalGrossRevenueB.String())
	addSummaryCell(doc, 153, 64, "Withholding", "tax claim", stmt.TotalWithHoldingTaxClaim.String())
	addSummaryCell(doc, 48, 89, "Total tax value of", "A, B, DA-1 and USA values", stmt.TotalTaxValue.String())
	addSummaryCell(doc, 83, 89, "Total gross income", "A-values", stmt.TotalGrossRevenueA.String())
	addSummaryCell(doc, 118, 89, "Total gross income", "B-values", stmt.TotalGrossRevenueB.String())
	addSummaryCell(doc, 153, 89, "Total gross income", "A, B, DA-1 and USA values", stmt.TotalGrossRevenueA.Add(stmt.TotalGrossRevenueB).String())
	doc.SetFont("Helvetica", "", 8)
	doc.Text(34, 128, `Values for the "Securities and balances" form`)
	doc.Text(34, 138, "If no foreign withholding tax credit is claimed,")
	doc.Text(34, 143, "use these total values in the securities and balances statement.")
}

func addSummaryCell(doc *gofpdf.Fpdf, x, y float64, l1, l2, value string) {
	doc.SetFont("Helvetica", "B", 6.8)
	doc.Text(x, y, l1)
	doc.Text(x, y+4, l2)
	doc.Line(x, y+9, x+22, y+9)
	doc.SetFont("Helvetica", "B", 8)
	doc.Text(x+16-doc.GetStringWidth(value), y+14, value)
}

func addSecurityTableHeader(doc *gofpdf.Fpdf, foreignTax bool) {
	doc.SetFillColor(210, 210, 210)
	doc.Rect(34, 55, 240, 13, "F")
	doc.SetFont("Helvetica", "", 6.5)
	doc.Text(35, 59, "Security no.")
	doc.Text(35, 63, "Date")
	doc.Text(52, 59, "Depot")
	doc.Text(52, 63, "Description / ISIN")
	doc.Text(128, 59, "Quantity")
	doc.Text(144, 59, "Currency")
	doc.Text(160, 59, "Unit price")
	doc.Text(182, 59, "FX rate")
	doc.Text(202, 59, "Tax value")
	if foreignTax {
		doc.Text(226, 59, "Gross income")
		doc.Text(252, 59, "Tax credit")
	} else {
		doc.Text(226, 59, "Gross income A")
		doc.Text(252, 59, "Gross income B")
	}
}

func addSecurityRow(doc *gofpdf.Fpdf, y float64, row reportRow, foreignTax bool) {
	if row.Shade {
		doc.SetFillColor(240, 240, 240)
		doc.Rect(34, y-3.2, 240, 4.8, "F")
	}
	style := ""
	if row.Bold {
		style = "B"
	}
	doc.SetFont("Helvetica", style, 6.6)
	doc.Text(35, y, displayDate(row.Date))
	doc.Text(52, y, truncateText(rowLabel(row), 62))
	rightText(doc, 138, y, row.Quantity)
	doc.Text(144, y, row.Currency)
	rightText(doc, 174, y, row.UnitPrice)
	rightText(doc, 196, y, row.FXRate)
	rightText(doc, 220, y, row.TaxValue)
	if foreignTax {
		rightText(doc, 248, y, firstNonEmpty(row.GrossB, row.GrossA))
		rightText(doc, 272, y, firstNonEmpty(row.WHT, row.USA))
	} else {
		rightText(doc, 248, y, row.GrossA)
		rightText(doc, 272, y, row.GrossB)
	}
}

func addBarcodeSegment(doc *gofpdf.Fpdf, segment Segment, x, y float64) error {
	code, err := pdf417ech.EncodeMacroSegment(pdf417ech.MacroSegment{
		FileID:       segment.MacroFileID,
		FileName:     segment.DocumentID,
		SegmentIndex: segment.Index,
		SegmentCount: segment.Total,
		Data:         segment.Data,
		Last:         segment.Last,
	})
	if err != nil {
		return fmt.Errorf("encoding PDF417 segment %d/%d: %w", segment.Index+1, segment.Total, err)
	}
	if code.Bounds().Dx() != barcodePixelWidth || code.Bounds().Dy() != barcodePixelHeight {
		return fmt.Errorf("PDF417 segment %d/%d image is %dx%d, want %dx%d", segment.Index+1, segment.Total, code.Bounds().Dx(), code.Bounds().Dy(), barcodePixelWidth, barcodePixelHeight)
	}
	img, err := pngBytes(scaleNearest(code, pdf417ImageWidth, pdf417ImageHeight))
	if err != nil {
		return fmt.Errorf("rendering PDF417 segment %d/%d: %w", segment.Index+1, segment.Total, err)
	}

	name := fmt.Sprintf("pdf417-%04d", segment.Index+1)
	opt := gofpdf.ImageOptions{ImageType: "png", ReadDpi: false}
	doc.RegisterImageOptionsReader(name, opt, bytes.NewReader(img))
	doc.ImageOptions(name, x, y, pdf417WidthMM, pdf417HeightMM, false, opt, 0, "")
	return nil
}

func addBarcodeSheetHeader(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement) {
	doc.SetTextColor(255, 0, 0)
	doc.SetFont("Helvetica", "", 8)
	doc.Text(34, 12, "Generated for tax purposes")
	doc.Text(92, 12, "eTax statement")
	doc.Text(168, 12, "Generated for tax purposes")
	doc.SetTextColor(0, 0, 0)
	doc.SetFont("Helvetica", "B", 24)
	doc.Text(34, 25, defaultInstitution(stmt))
	doc.SetFont("Helvetica", "B", 16)
	doc.Text(34, 47, fmt.Sprintf("Tax statement 31.12.%d", stmt.TaxPeriod))
	doc.SetFont("Helvetica", "B", 14)
	doc.Text(34, 56, "Barcode pages")

	doc.Line(154, 17, 204, 17)
	doc.Line(154, 54, 204, 54)
	addHeaderMeta(doc, stmt, 154, 23)
}

func addBarcodeSheetFooter(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement, page, totalPages int) {
	_, pageHeight := doc.GetPageSize()
	doc.SetFont("Helvetica", "", 7)
	doc.Text(34, pageHeight-15, footerInstitution(stmt))
	doc.Text(258, pageHeight-15, fmt.Sprintf("Page %d of %d", page, totalPages))
}

func addPageMarker(doc *gofpdf.Fpdf, stmt ech0196.TaxStatement, page int, hasPDF417 bool, x, y float64) error {
	content := pageMarkerContent(stmt, page, hasPDF417)
	code, err := code128.Encode(content)
	if err != nil {
		return fmt.Errorf("encoding page marker: %w", err)
	}
	scaled, err := barcode.Scale(code, code.Bounds().Dx(), 400)
	if err != nil {
		return fmt.Errorf("scaling page marker: %w", err)
	}
	img, err := rotatedPNG(scaled)
	if err != nil {
		return fmt.Errorf("rendering page marker: %w", err)
	}
	name := fmt.Sprintf("page-marker-%04d", page)
	opt := gofpdf.ImageOptions{ImageType: "png", ReadDpi: false}
	doc.RegisterImageOptionsReader(name, opt, bytes.NewReader(img))
	doc.ImageOptions(name, x, y, pageMarkerWidthMM, pageMarkerHeightMM, false, opt, 0, "")
	doc.TransformBegin()
	doc.TransformRotate(90, x-2, y+pageMarkerHeightMM)
	doc.SetFont("Helvetica", "", 7)
	doc.Text(x-2, y+pageMarkerHeightMM, content)
	doc.TransformEnd()
	return nil
}

func rotatedPNG(src image.Image) ([]byte, error) {
	rotated := rotateClockwise(src)
	return pngBytes(rotated)
}

func pngBytes(src image.Image) ([]byte, error) {
	rgba := image.NewRGBA(src.Bounds())
	draw.Draw(rgba, rgba.Bounds(), src, src.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rotateClockwise(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.Y-y-1, x-b.Min.X, src.At(x, y))
		}
	}
	return dst
}

func scaleNearest(src image.Image, width, height int) *image.RGBA {
	srcBounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + (y*srcBounds.Dy())/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + (x*srcBounds.Dx())/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func compressXML(xmlData []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(xmlData); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func countSegments(documentID, fileName string, data []byte, total int) int {
	count := 0
	for offset := 0; offset < len(data) || (len(data) == 0 && count == 0); count++ {
		remaining := len(data) - offset
		lastCap := pdf417ech.MaxSegmentBytes(documentID, fileName, count, total, true)
		if remaining <= lastCap {
			return count + 1
		}
		offset += pdf417ech.MaxSegmentBytes(documentID, fileName, count, total, false)
	}
	return count
}

func macroFileID(documentID string) string {
	sum := crc32.ChecksumIEEE([]byte(documentID))
	return fmt.Sprintf("%03d%03d%03d%03d",
		100+byte(sum>>24)%156,
		100+byte(sum>>16)%156,
		100+byte(sum>>8)%156,
		100+byte(sum)%156,
	)
}

func pageMarkerContent(stmt ech0196.TaxStatement, page int, hasPDF417 bool) string {
	if page < 0 {
		page = 0
	}
	if page > 999 {
		page = 999
	}
	barcodePage := 0
	if hasPDF417 {
		barcodePage = 1
	}
	return fmt.Sprintf("196%02d%s%03d%d02", stmt.MinorVersion, organisationNumber(stmt), page, barcodePage)
}

func organisationNumber(stmt ech0196.TaxStatement) string {
	if len(stmt.ID) >= 7 {
		candidate := stmt.ID[2:7]
		if allDigits(candidate) {
			return candidate
		}
	}
	return "00000"
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func writeLine(doc *gofpdf.Fpdf, label, value string) {
	doc.CellFormat(48, 6, label+":", "", 0, "", false, 0, "")
	doc.CellFormat(0, 6, value, "", 1, "", false, 0, "")
}

func rowsForSecurities(stmt ech0196.TaxStatement, include func(ech0196.Security) bool, foreignTax bool) []reportRow {
	var rows []reportRow
	for _, depot := range stmt.ListOfSecurities.Depots {
		depotAdded := false
		for _, sec := range depot.Securities {
			if !include(sec) {
				continue
			}
			if !depotAdded {
				rows = append(rows, reportRow{Label: "Depot " + depot.DepotNumber, Bold: true})
				depotAdded = true
			}
			rows = append(rows, reportRow{Label: sec.SecurityName, ISIN: sec.ISIN, Bold: true})
			for _, stock := range sortedStocks(sec.Stocks) {
				rows = append(rows, reportRow{
					Date:      stock.ReferenceDate,
					Label:     stockName(stock),
					Quantity:  signedQuantity(stock.Mutation, stock.Quantity.String()),
					Currency:  stock.BalanceCurrency,
					UnitPrice: stock.UnitPrice.String(),
					FXRate:    stock.ExchangeRate.String(),
				})
			}
			for _, payment := range sortedPayments(sec.Payments) {
				if foreignTax && payment.WithHoldingTaxClaim.IsZero() && payment.AdditionalWithHoldingUSA.IsZero() {
					continue
				}
				rows = append(rows, reportRow{
					Date:      payment.PaymentDate,
					Label:     firstNonEmpty(payment.Name, "Income"),
					Quantity:  payment.Quantity.String(),
					Currency:  payment.AmountCurrency,
					UnitPrice: payment.Amount.String(),
					FXRate:    payment.ExchangeRate.String(),
					GrossA:    nonZero(payment.GrossRevenueA.String()),
					GrossB:    nonZero(payment.GrossRevenueB.String()),
					WHT:       nonZero(payment.WithHoldingTaxClaim.String()),
					USA:       nonZero(payment.AdditionalWithHoldingUSA.String()),
				})
			}
			if sec.TaxValue != nil {
				rows = append(rows, reportRow{
					Date:      sec.TaxValue.ReferenceDate,
					Label:     "Balance / tax value / income",
					Quantity:  sec.TaxValue.Quantity.String(),
					Currency:  sec.TaxValue.BalanceCurrency,
					UnitPrice: sec.TaxValue.UnitPrice.String(),
					FXRate:    sec.TaxValue.ExchangeRate.String(),
					TaxValue:  sec.TaxValue.Value.String(),
					GrossA:    sumGrossA(sec.Payments),
					GrossB:    sumGrossB(sec.Payments),
					WHT:       sumWHT(sec.Payments),
					USA:       sumUSA(sec.Payments),
					Bold:      true,
					Shade:     true,
				})
			}
		}
	}
	return rows
}

func sortedStocks(stocks []ech0196.Stock) []ech0196.Stock {
	out := append([]ech0196.Stock(nil), stocks...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ReferenceDate < out[j].ReferenceDate
	})
	return out
}

func sortedPayments(payments []ech0196.Payment) []ech0196.Payment {
	out := append([]ech0196.Payment(nil), payments...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PaymentDate < out[j].PaymentDate
	})
	return out
}

func stockName(stock ech0196.Stock) string {
	if stock.Name != "" {
		switch strings.ToUpper(stock.Name) {
		case "BUY":
			return "Buy"
		case "SELL":
			return "Sell"
		}
		return stock.Name
	}
	if stock.Mutation {
		return "Mutation"
	}
	return "Balance"
}

func signedQuantity(mutation bool, value string) string {
	if !mutation || value == "" || strings.HasPrefix(value, "-") {
		return value
	}
	return "+ " + value
}

func sumGrossA(payments []ech0196.Payment) string {
	var sum stringDecimal
	for _, p := range payments {
		sum.add(p.GrossRevenueA.String())
	}
	return nonZero(sum.String())
}

func sumGrossB(payments []ech0196.Payment) string {
	var sum stringDecimal
	for _, p := range payments {
		sum.add(p.GrossRevenueB.String())
	}
	return nonZero(sum.String())
}

func sumWHT(payments []ech0196.Payment) string {
	var sum stringDecimal
	for _, p := range payments {
		sum.add(p.WithHoldingTaxClaim.String())
	}
	return nonZero(sum.String())
}

func sumUSA(payments []ech0196.Payment) string {
	var sum stringDecimal
	for _, p := range payments {
		sum.add(p.AdditionalWithHoldingUSA.String())
	}
	return nonZero(sum.String())
}

type stringDecimal struct {
	value float64
}

func (d *stringDecimal) add(s string) {
	var v float64
	_, _ = fmt.Sscanf(s, "%f", &v)
	d.value += v
}

func (d stringDecimal) String() string {
	if d.value == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", d.value)
}

func rightText(doc *gofpdf.Fpdf, x, y float64, value string) {
	if value == "" {
		return
	}
	doc.Text(x-doc.GetStringWidth(value), y, value)
}

func rowLabel(row reportRow) string {
	if row.ISIN == "" {
		return row.Label
	}
	return row.Label + " / " + row.ISIN
}

func displayDate(s string) string {
	if len(s) == len("2006-01-02") {
		return s[8:10] + "." + s[5:7] + "." + s[:4]
	}
	return s
}

func nonZero(s string) string {
	if s == "0" || s == "0.00" || s == "" {
		return ""
	}
	return s
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func reportRowsPerPage() int {
	return 22
}

func clientName(stmt ech0196.TaxStatement) string {
	if len(stmt.Clients) == 0 {
		return ""
	}
	return strings.TrimSpace(stmt.Clients[0].FirstName + " " + stmt.Clients[0].LastName)
}

func clientNumber(stmt ech0196.TaxStatement) string {
	if len(stmt.Clients) == 0 {
		return ""
	}
	return stmt.Clients[0].ClientNumber
}

func defaultInstitution(stmt ech0196.TaxStatement) string {
	if stmt.Institution.Name != "" {
		return stmt.Institution.Name
	}
	return "Reference bank"
}

func footerInstitution(stmt ech0196.TaxStatement) string {
	name := defaultInstitution(stmt)
	if stmt.Institution.LEI != "" {
		return name + ", LEI " + stmt.Institution.LEI
	}
	return name
}

func creationDate(s string) string {
	if len(s) >= len("2006-01-02") {
		return s[8:10] + "." + s[5:7] + "." + s[:4]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
