package kursliste

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLooksUpSecurityByISIN(t *testing.T) {
	store, err := Load("testdata/kursliste_2025.xml", 2025)
	if err != nil {
		t.Fatal(err)
	}

	apple, ok := store.LookupISIN("us0378331005")
	if !ok {
		t.Fatal("Apple not found by ISIN")
	}
	if apple.ValorNumber != "37833100" {
		t.Fatalf("valorNumber = %q, want 37833100", apple.ValorNumber)
	}
	if apple.SecurityGroup != "SHARE" || apple.SecurityType != "SHARE.COMMON" {
		t.Fatalf("security type = %q/%q", apple.SecurityGroup, apple.SecurityType)
	}
	if apple.SecurityName != "Apple Inc" || apple.Country != "US" || apple.Currency != "USD" {
		t.Fatalf("unexpected security metadata: %+v", apple)
	}

	vt, ok := store.LookupISIN("US9220427424")
	if !ok {
		t.Fatal("fund not found by ISIN")
	}
	if vt.SecurityName != "Vanguard Total World Stock ETF" {
		t.Fatalf("fund securityName = %q", vt.SecurityName)
	}
}

func TestLoadForISINsOnlyStoresRequestedSecurities(t *testing.T) {
	store, err := LoadForISINs("testdata/kursliste_2025.xml", 2025, []string{"US0378331005"})
	if err != nil {
		t.Fatal(err)
	}
	if store.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", store.Len())
	}
	if _, ok := store.LookupISIN("US0378331005"); !ok {
		t.Fatal("requested security not found")
	}
	if _, ok := store.LookupISIN("US9220427424"); ok {
		t.Fatal("unrequested security was loaded")
	}
}

func TestLoadDirectoryFiltersByYear(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, "testdata/kursliste_2025.xml", filepath.Join(dir, "kursliste_2025.xml"))
	if err := os.WriteFile(filepath.Join(dir, "kursliste_2024.xml"), []byte(`<kursliste year="2024"><share valorNumber="123" isin="CH0012221716"/></kursliste>`), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := Load(dir, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if store.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", store.Len())
	}
	if _, ok := store.LookupISIN("CH0012221716"); ok {
		t.Fatal("loaded security from wrong year")
	}
}

func TestLoadSupportsLatin1KurslisteXML(t *testing.T) {
	xml := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><kursliste year=\"2025\"><share valorNumber=\"123456\" isin=\"CH0012221716\" securityName=\"Z\xFCrich\"/></kursliste>")
	store := NewStore()
	if err := store.LoadReader(strings.NewReader(string(xml)), 2025); err != nil {
		t.Fatal(err)
	}
	sec, ok := store.LookupISIN("CH0012221716")
	if !ok {
		t.Fatal("security not found")
	}
	if sec.SecurityName != "Zürich" {
		t.Fatalf("SecurityName = %q, want Zürich", sec.SecurityName)
	}
}

func TestLoadFileRejectsWrongYear(t *testing.T) {
	_, err := Load("testdata/kursliste_2025.xml", 2024)
	if err == nil {
		t.Fatal("expected year mismatch")
	}
	if !strings.Contains(err.Error(), "year mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatal(err)
	}
}
