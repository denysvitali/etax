package ech0196_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"etax/internal/ech0196"
	"etax/internal/provider/ibkr"
)

func TestIBKRToECH0196Sample(t *testing.T) {
	f, err := os.Open("../../testdata/ibkr_sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	report, err := ibkr.New().Parse(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := ech0196.FromReport(report, "ZH", 2025, ech0196.ClientInfo{FirstName: "Max", LastName: "Muster"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := ech0196.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}

	xml := string(data)
	for _, want := range []string{
		`<listOfSecurities`,
		`isin="US0378331005"`,
		`totalTaxValue="17160"`,
		`totalGrossRevenueB="22"`,
		`additionalWithHoldingTaxUSA="3.3"`,
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("generated XML missing %q:\n%s", want, xml)
		}
	}
}
