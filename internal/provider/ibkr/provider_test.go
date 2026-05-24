package ibkr_test

import (
	"context"
	"os"
	"testing"

	"etax/internal/domain"
	"etax/internal/money"
	"etax/internal/provider/ibkr"
)

func TestParseNormalizesIBKRFlexStatement(t *testing.T) {
	f, err := os.Open("testdata/normalization.flex")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	report, err := ibkr.New().Parse(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}

	if report.ProviderID != "ibkr" {
		t.Fatalf("ProviderID = %q, want ibkr", report.ProviderID)
	}
	if report.AccountID != "U7654321" {
		t.Fatalf("AccountID = %q, want U7654321", report.AccountID)
	}
	if report.BaseCurrency != "CHF" {
		t.Fatalf("BaseCurrency = %q, want CHF", report.BaseCurrency)
	}
	if report.PeriodFrom != "2025-01-01" || report.PeriodTo != "2025-12-31" {
		t.Fatalf("period = %s..%s, want 2025-01-01..2025-12-31", report.PeriodFrom, report.PeriodTo)
	}

	assertPositions(t, report)
	assertTrades(t, report)
	assertCashFlows(t, report)
}

func assertPositions(t *testing.T, report *domain.Report) {
	t.Helper()

	if len(report.Positions) != 2 {
		t.Fatalf("len(Positions) = %d, want 2", len(report.Positions))
	}

	usd := report.Positions[0]
	if usd.Symbol != "AAPL" || usd.Name != "APPLE INC" || usd.ISIN != "US0378331005" {
		t.Fatalf("USD position security = %#v", usd)
	}
	if usd.AssetCategory != "STK" || usd.ReferenceDate != "2025-12-31" || usd.Currency != "USD" {
		t.Fatalf("USD position metadata = %#v", usd)
	}
	assertDecimal(t, usd.Quantity, "100", "USD position quantity")
	assertDecimal(t, usd.UnitPrice, "195", "USD position unit price")
	assertDecimal(t, usd.Value, "17160", "USD position CHF value")
	assertDecimal(t, usd.FXToCHF, "0.88", "USD position FXToCHF")

	chf := report.Positions[1]
	if chf.Symbol != "CHSPI" || chf.Name != "CHSPI" || chf.ISIN != "CH0000000001" {
		t.Fatalf("CHF position security = %#v", chf)
	}
	if chf.ReferenceDate != "2025-12-31" || chf.Currency != "CHF" {
		t.Fatalf("CHF position metadata = %#v", chf)
	}
	assertDecimal(t, chf.Value, "1200.5", "CHF position CHF value")
	assertDecimal(t, chf.FXToCHF, "1", "CHF position FXToCHF")
}

func assertTrades(t *testing.T, report *domain.Report) {
	t.Helper()

	if len(report.Trades) != 2 {
		t.Fatalf("len(Trades) = %d, want 2", len(report.Trades))
	}

	buy := report.Trades[0]
	if buy.Symbol != "AAPL" || buy.Name != "APPLE INC" || buy.ISIN != "US0378331005" {
		t.Fatalf("buy trade security = %#v", buy)
	}
	if buy.Date != "2025-03-15" || buy.Side != "BUY" || buy.Currency != "USD" {
		t.Fatalf("buy trade metadata = %#v", buy)
	}
	assertDecimal(t, buy.Quantity, "50", "buy trade absolute quantity")
	assertDecimal(t, buy.Price, "170", "buy trade price")
	assertDecimal(t, buy.Value, "7480", "buy trade CHF value")
	assertDecimal(t, buy.FXToCHF, "0.88", "buy trade FXToCHF")

	sell := report.Trades[1]
	if sell.Symbol != "AAPL" || sell.Date != "2025-08-20" || sell.Side != "SELL" {
		t.Fatalf("sell trade metadata = %#v", sell)
	}
	assertDecimal(t, sell.Quantity, "5", "sell trade absolute quantity")
	assertDecimal(t, sell.Value, "792", "sell trade CHF value")
}

func assertCashFlows(t *testing.T, report *domain.Report) {
	t.Helper()

	if len(report.CashFlows) != 3 {
		t.Fatalf("len(CashFlows) = %d, want 3", len(report.CashFlows))
	}

	dividend := report.CashFlows[0]
	if dividend.Type != "Dividends" || dividend.Symbol != "AAPL" || dividend.Name != "APPLE INC" {
		t.Fatalf("dividend metadata = %#v", dividend)
	}
	if dividend.Description != "AAPL CASH DIVIDEND" || dividend.Date != "2025-05-15" {
		t.Fatalf("dividend description/date = %#v", dividend)
	}
	assertDecimal(t, dividend.Amount, "25", "dividend amount")
	assertDecimal(t, dividend.FXToCHF, "0.88", "dividend FXToCHF")

	withholding := report.CashFlows[1]
	if withholding.Type != "WithholdingTax" || withholding.Symbol != "AAPL" || withholding.Name != "APPLE INC" {
		t.Fatalf("withholding tax metadata = %#v", withholding)
	}
	if withholding.Description != "AAPL CASH DIVIDEND - US TAX" || withholding.Date != "2025-05-15" {
		t.Fatalf("withholding tax description/date = %#v", withholding)
	}
	assertDecimal(t, withholding.Amount, "-3.75", "withholding tax amount")
	assertDecimal(t, withholding.FXToCHF, "0.88", "withholding tax FXToCHF")

	chfInterest := report.CashFlows[2]
	if chfInterest.Type != "Interest" || chfInterest.Currency != "CHF" || chfInterest.Date != "2025-06-01" {
		t.Fatalf("CHF cash flow metadata = %#v", chfInterest)
	}
	assertDecimal(t, chfInterest.Amount, "1.25", "CHF cash flow amount")
	assertDecimal(t, chfInterest.FXToCHF, "1", "CHF cash flow FXToCHF")
}

func assertDecimal(t *testing.T, got money.Decimal, want, field string) {
	t.Helper()

	if got.String() != want {
		t.Fatalf("%s = %s, want %s", field, got.String(), want)
	}
}
