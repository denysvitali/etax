package domain

import (
	"slices"
	"testing"
)

func TestReportISINs(t *testing.T) {
	report := &Report{
		Positions: []Position{
			{ISIN: "US0378331005"},
			{ISIN: ""},
			{ISIN: "CH0012032048"},
		},
		Trades: []Trade{
			{ISIN: "US0378331005"},
			{ISIN: "US5949181045"},
		},
		CashFlows: []CashFlow{
			{ISIN: "CH0012032048"},
			{ISIN: "US02079K3059"},
		},
	}

	want := []string{"US0378331005", "CH0012032048", "US5949181045", "US02079K3059"}
	if got := report.ISINs(); !slices.Equal(got, want) {
		t.Fatalf("ISINs() = %#v, want %#v", got, want)
	}
}

func TestReportISINsNilReport(t *testing.T) {
	var report *Report
	if got := report.ISINs(); got != nil {
		t.Fatalf("ISINs() = %#v, want nil", got)
	}
}
