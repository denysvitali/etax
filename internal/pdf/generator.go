package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"strconv"
	"strings"

	"etax/internal/ech0196"
)

func WriteSummary(path string, stmt ech0196.TaxStatement, xmlData []byte) error {
	data, err := Bytes(stmt, xmlData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Bytes(stmt ech0196.TaxStatement, xmlData []byte) ([]byte, error) {
	lines := []string{
		"E-Steuerauszug / Releve fiscal electronique",
		"Institution: " + stmt.Institution.Name,
		"LEI: " + stmt.Institution.LEI,
		fmt.Sprintf("Steuerperiode: %d", stmt.TaxPeriod),
		"Kanton: " + stmt.Canton,
		"Total tax value CHF: " + stmt.TotalTaxValue.String(),
		"Gross revenue A CHF: " + stmt.TotalGrossRevenueA.String(),
		"Gross revenue B CHF: " + stmt.TotalGrossRevenueB.String(),
		"Withholding tax claim CHF: " + stmt.TotalWithHoldingTaxClaim.String(),
		"",
		"Embedded XML preview follows. Use the generated XML file for strict eCH import.",
	}
	if len(stmt.Clients) > 0 {
		c := stmt.Clients[0]
		lines = append(lines[:5], append([]string{"Client: " + strings.TrimSpace(c.FirstName+" "+c.LastName), "Account: " + c.ClientNumber}, lines[5:]...)...)
	}
	for _, line := range strings.Split(string(xmlData), "\n") {
		if len(lines) > 42 {
			break
		}
		lines = append(lines, truncate(line, 100))
	}

	content := textContent(lines)
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write([]byte(content)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", compressed.Len(), compressed.String()),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i < len(offsets); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes(), nil
}

func textContent(lines []string) string {
	var b strings.Builder
	b.WriteString("BT\n/F1 11 Tf\n50 800 Td\n14 TL\n")
	for _, line := range lines {
		b.WriteString("(")
		b.WriteString(escape(line))
		b.WriteString(") Tj\nT*\n")
	}
	b.WriteString("ET\n")
	return b.String()
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)
	return strconv.QuoteToASCII(s)[1 : len(strconv.QuoteToASCII(s))-1]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
