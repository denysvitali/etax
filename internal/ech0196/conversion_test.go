package ech0196_test

import (
	"context"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/denysvitali/etax/internal/domain"
	"github.com/denysvitali/etax/internal/ech0196"
	"github.com/denysvitali/etax/internal/kursliste"
	"github.com/denysvitali/etax/internal/money"
	"github.com/denysvitali/etax/internal/provider/ibkr"
)

func TestIBKRToECH0196Sample(t *testing.T) {
	data := marshalIBKRSample(t)

	var got sampleTaxStatement
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("generated eCH XML is not well-formed: %v\n%s", err, data)
	}

	if got.XMLName.Local != "taxStatement" {
		t.Fatalf("root element = %q, want taxStatement", got.XMLName.Local)
	}
	if got.XMLName.Space != ech0196.NamespaceECH0196 {
		t.Fatalf("root namespace = %q, want %q", got.XMLName.Space, ech0196.NamespaceECH0196)
	}

	assertEqual(t, "id", got.ID, "CH19334000000U12345672025123101")
	assertEqual(t, "minorVersion", got.MinorVersion, ech0196.MinorVersion)
	if got.CreationDate == "" {
		t.Fatal("creationDate is empty")
	}
	assertEqual(t, "taxPeriod", got.TaxPeriod, 2025)
	assertEqual(t, "periodFrom", got.PeriodFrom, "2025-01-01")
	assertEqual(t, "periodTo", got.PeriodTo, "2025-12-31")
	assertEqual(t, "country", got.Country, "CH")
	assertEqual(t, "canton", got.Canton, "ZH")
	assertEqual(t, "root totalTaxValue", got.TotalTaxValue, "17160")
	assertEqual(t, "root totalGrossRevenueA", got.TotalGrossRevenueA, "0")
	assertEqual(t, "root totalGrossRevenueB", got.TotalGrossRevenueB, "22")
	assertEqual(t, "root totalWithHoldingTaxClaim", got.TotalWithHoldingTaxClaim, "0")

	assertEqual(t, "institution name", got.Institution.Name, "Interactive Brokers")
	assertEqual(t, "institution lei", got.Institution.LEI, "5493004J90J71E0E4R31")
	if len(got.Clients) != 1 {
		t.Fatalf("client count = %d, want 1", len(got.Clients))
	}
	assertEqual(t, "client number", got.Clients[0].ClientNumber, "U1234567")
	assertEqual(t, "client firstName", got.Clients[0].FirstName, "Max")
	assertEqual(t, "client lastName", got.Clients[0].LastName, "Muster")

	list := got.ListOfSecurities
	assertEqual(t, "listOfSecurities totalTaxValue", list.TotalTaxValue, got.TotalTaxValue)
	assertEqual(t, "listOfSecurities totalGrossRevenueA", list.TotalGrossRevenueA, got.TotalGrossRevenueA)
	assertEqual(t, "listOfSecurities totalGrossRevenueB", list.TotalGrossRevenueB, got.TotalGrossRevenueB)
	assertEqual(t, "listOfSecurities totalWithHoldingTaxClaim", list.TotalWithHoldingTaxClaim, got.TotalWithHoldingTaxClaim)
	assertEqual(t, "listOfSecurities totalLumpSumTaxCredit", list.TotalLumpSumTaxCredit, "0.00")
	assertEqual(t, "listOfSecurities totalNonRecoverableTax", list.TotalNonRecoverableTax, "0.00")
	assertEqual(t, "listOfSecurities totalAdditionalWithHoldingTaxUSA", list.TotalAdditionalWithHoldingTaxUSA, "3.3")
	assertEqual(t, "listOfSecurities totalGrossRevenueIUP", list.TotalGrossRevenueIUP, "0.00")
	assertEqual(t, "listOfSecurities totalGrossRevenueConversion", list.TotalGrossRevenueConversion, "0.00")
	if len(list.Depots) != 1 {
		t.Fatalf("depot count = %d, want 1", len(list.Depots))
	}

	depot := list.Depots[0]
	assertEqual(t, "depot number", depot.DepotNumber, "U1234567")
	if len(depot.Securities) != 1 {
		t.Fatalf("security count = %d, want 1", len(depot.Securities))
	}

	security := depot.Securities[0]
	assertEqual(t, "security positionId", security.PositionID, 1)
	assertEqual(t, "security isin", security.ISIN, "US0378331005")
	assertEqual(t, "security country", security.Country, "US")
	assertEqual(t, "security currency", security.Currency, "USD")
	assertEqual(t, "security quotationType", security.QuotationType, "PIECE")
	assertEqual(t, "security nominalValue", security.NominalValue, "1")
	assertEqual(t, "security category", security.SecurityCategory, "SHARE")
	assertEqual(t, "security type", security.SecurityType, "SHARE.COMMON")
	assertEqual(t, "security name", security.SecurityName, "APPLE INC")

	assertEqual(t, "taxValue referenceDate", security.TaxValue.ReferenceDate, "2025-12-31")
	assertEqual(t, "taxValue quotationType", security.TaxValue.QuotationType, "PIECE")
	assertEqual(t, "taxValue quantity", security.TaxValue.Quantity, "100")
	assertEqual(t, "taxValue balanceCurrency", security.TaxValue.BalanceCurrency, "USD")
	assertEqual(t, "taxValue unitPrice", security.TaxValue.UnitPrice, "195")
	assertEqual(t, "taxValue value", security.TaxValue.Value, "17160")
	assertEqual(t, "taxValue exchangeRate", security.TaxValue.ExchangeRate, "0.88")

	if len(security.Payments) != 1 {
		t.Fatalf("payment count = %d, want 1", len(security.Payments))
	}
	payment := security.Payments[0]
	assertEqual(t, "payment date", payment.PaymentDate, "2025-05-15")
	assertEqual(t, "payment name", payment.Name, "AAPL CASH DIVIDEND")
	assertEqual(t, "payment quotationType", payment.QuotationType, "PIECE")
	assertEqual(t, "payment quantity", payment.Quantity, "0")
	assertEqual(t, "payment amountCurrency", payment.AmountCurrency, "USD")
	assertEqual(t, "payment amount", payment.Amount, "25")
	assertEqual(t, "payment exchangeRate", payment.ExchangeRate, "0.88")
	assertEqual(t, "payment grossRevenueA", payment.GrossRevenueA, "0.00")
	assertEqual(t, "payment grossRevenueB", payment.GrossRevenueB, "22")
	assertEqual(t, "payment withHoldingTaxClaim", payment.WithHoldingTaxClaim, "0.00")
	assertEqual(t, "payment additionalWithHoldingTaxUSA", payment.AdditionalWithHoldingTaxUSA, "3.3")

	if len(security.Stocks) != 2 {
		t.Fatalf("stock count = %d, want 2 year-end/trade entries", len(security.Stocks))
	}
	assertEqual(t, "year-end stock date", security.Stocks[0].ReferenceDate, "2025-12-31")
	assertEqual(t, "year-end stock mutation", security.Stocks[0].Mutation, "false")
	assertEqual(t, "trade stock date", security.Stocks[1].ReferenceDate, "2025-03-15")
	assertEqual(t, "trade stock mutation", security.Stocks[1].Mutation, "true")
}

func TestIBKRToECH0196SampleWithKurslisteEnrichment(t *testing.T) {
	f, err := os.Open("../../testdata/ibkr_sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	report, err := ibkr.New().Parse(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	store, err := kursliste.Load("../kursliste/testdata/kursliste_2025.xml", 2025)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := ech0196.FromReportWithOptions(report, "ZH", 2025, ech0196.ClientInfo{FirstName: "Max", LastName: "Muster"}, ech0196.Options{Kursliste: store})
	if err != nil {
		t.Fatal(err)
	}
	data, err := ech0196.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}

	var got sampleTaxStatement
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("generated eCH XML is not well-formed: %v\n%s", err, data)
	}
	security := got.ListOfSecurities.Depots[0].Securities[0]
	assertEqual(t, "security valorNumber", security.ValorNumber, "37833100")
	assertEqual(t, "security name", security.SecurityName, "APPLE INC")
	assertEqual(t, "security category", security.SecurityCategory, "SHARE")
	assertEqual(t, "security type", security.SecurityType, "SHARE.COMMON")
}

func TestFromReportRejectsPeriodOutsideTaxYear(t *testing.T) {
	_, err := ech0196.FromReport(&domain.Report{
		ProviderID: "ibkr",
		AccountID:  "U1234567",
		PeriodFrom: "2024-01-01",
		PeriodTo:   "2024-12-31",
	}, "ZH", 2025, ech0196.ClientInfo{})
	if err == nil {
		t.Fatal("expected period mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match tax year 2025") {
		t.Fatalf("error = %v", err)
	}
}

func TestFromReportKeepsSameDayDividendCashflowsDistinct(t *testing.T) {
	report := &domain.Report{
		ProviderID:      "test",
		AccountID:       "U123",
		PeriodFrom:      "2025-01-01",
		PeriodTo:        "2025-12-31",
		InstitutionName: "Test Broker",
		Positions: []domain.Position{{
			ISIN:          "US01609W1027",
			Name:          "ALIBABA GROUP HOLDING-SP ADR",
			AssetCategory: "STK",
			Quantity:      money.Must("32.8521"),
			UnitPrice:     money.Must("146.58"),
			Value:         money.Must("3817"),
			Currency:      "USD",
			FXToCHF:       money.Must("0.79268"),
			ReferenceDate: "2025-12-31",
		}},
		CashFlows: []domain.CashFlow{
			{Type: "Dividends", ISIN: "US01609W1027", Name: "ALIBABA GROUP HOLDING-SP ADR", Description: "BABA(US01609W1027) Cash Dividend USD 1.05 per Share (Ordinary Dividend)", Amount: money.Must("13.49"), Currency: "USD", FXToCHF: money.Must("0.79649"), Date: "2025-07-10"},
			{Type: "Dividends", ISIN: "US01609W1027", Name: "ALIBABA GROUP HOLDING-SP ADR", Description: "BABA(US01609W1027) Cash Dividend USD 0.95 per Share (Bonus Dividend)", Amount: money.Must("12.21"), Currency: "USD", FXToCHF: money.Must("0.79649"), Date: "2025-07-10"},
		},
	}

	stmt, err := ech0196.FromReport(report, "ZH", 2025, ech0196.ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	payments := stmt.ListOfSecurities.Depots[0].Securities[0].Payments
	if len(payments) != 2 {
		t.Fatalf("payment count = %d, want 2 distinct payments", len(payments))
	}
	assertEqual(t, "first payment amount", payments[0].Amount.String(), "12.21")
	assertEqual(t, "first payment grossRevenueB", payments[0].GrossRevenueB.String(), "9.725143")
	assertEqual(t, "second payment amount", payments[1].Amount.String(), "13.49")
	assertEqual(t, "second payment grossRevenueB", payments[1].GrossRevenueB.String(), "10.74465")
}

func TestIBKRToECH0196SampleValidatesAgainstOfficialXSD(t *testing.T) {
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skipf("xmllint unavailable: %v", err)
	}
	data := marshalIBKRSample(t)
	xmlPath := filepath.Join(t.TempDir(), "taxstatement.xml")
	if err := os.WriteFile(xmlPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ech0196.ValidateXML(xmlPath, "../../schemas/eCH-0196-2-2.xsd"); err != nil {
		t.Fatalf("generated XML does not validate against eCH-0196 XSD: %v\n%s", err, data)
	}
}

func TestSyntheticECH2024Fixture(t *testing.T) {
	f, err := os.Open("../../testdata/synthetic_ech_2024.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var got sampleTaxStatement
	if err := xml.NewDecoder(f).Decode(&got); err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "root element", got.XMLName.Local, "taxStatement")
	assertEqual(t, "root namespace", got.XMLName.Space, ech0196.NamespaceECH0196)
	assertEqual(t, "minorVersion", got.MinorVersion, 21)
	assertEqual(t, "taxPeriod", got.TaxPeriod, 2024)
	assertEqual(t, "periodFrom", got.PeriodFrom, "2024-01-01")
	assertEqual(t, "periodTo", got.PeriodTo, "2024-12-31")
	assertEqual(t, "canton", got.Canton, "ZH")
	if got.TotalTaxValue == "" || got.TotalGrossRevenueB == "" {
		t.Fatal("synthetic fixture is missing root totals")
	}
}

func marshalIBKRSample(t *testing.T) []byte {
	t.Helper()

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

	return data
}

func assertEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

type sampleTaxStatement struct {
	XMLName                  xml.Name
	ID                       string `xml:"id,attr"`
	MinorVersion             int    `xml:"minorVersion,attr"`
	CreationDate             string `xml:"creationDate,attr"`
	TaxPeriod                int    `xml:"taxPeriod,attr"`
	PeriodFrom               string `xml:"periodFrom,attr"`
	PeriodTo                 string `xml:"periodTo,attr"`
	Country                  string `xml:"country,attr"`
	Canton                   string `xml:"canton,attr"`
	TotalTaxValue            string `xml:"totalTaxValue,attr"`
	TotalGrossRevenueA       string `xml:"totalGrossRevenueA,attr"`
	TotalGrossRevenueB       string `xml:"totalGrossRevenueB,attr"`
	TotalWithHoldingTaxClaim string `xml:"totalWithHoldingTaxClaim,attr"`
	Institution              struct {
		Name string `xml:"name,attr"`
		LEI  string `xml:"lei,attr"`
	} `xml:"institution"`
	Clients          []sampleClient   `xml:"client"`
	ListOfSecurities sampleSecurities `xml:"listOfSecurities"`
}

type sampleClient struct {
	ClientNumber string `xml:"clientNumber,attr"`
	FirstName    string `xml:"firstName,attr"`
	LastName     string `xml:"lastName,attr"`
}

type sampleSecurities struct {
	TotalTaxValue                    string        `xml:"totalTaxValue,attr"`
	TotalGrossRevenueA               string        `xml:"totalGrossRevenueA,attr"`
	TotalGrossRevenueB               string        `xml:"totalGrossRevenueB,attr"`
	TotalWithHoldingTaxClaim         string        `xml:"totalWithHoldingTaxClaim,attr"`
	TotalLumpSumTaxCredit            string        `xml:"totalLumpSumTaxCredit,attr"`
	TotalNonRecoverableTax           string        `xml:"totalNonRecoverableTax,attr"`
	TotalAdditionalWithHoldingTaxUSA string        `xml:"totalAdditionalWithHoldingTaxUSA,attr"`
	TotalGrossRevenueIUP             string        `xml:"totalGrossRevenueIUP,attr"`
	TotalGrossRevenueConversion      string        `xml:"totalGrossRevenueConversion,attr"`
	Depots                           []sampleDepot `xml:"depot"`
}

type sampleDepot struct {
	DepotNumber string           `xml:"depotNumber,attr"`
	Securities  []sampleSecurity `xml:"security"`
}

type sampleSecurity struct {
	PositionID       int             `xml:"positionId,attr"`
	ValorNumber      string          `xml:"valorNumber,attr"`
	ISIN             string          `xml:"isin,attr"`
	Country          string          `xml:"country,attr"`
	Currency         string          `xml:"currency,attr"`
	QuotationType    string          `xml:"quotationType,attr"`
	NominalValue     string          `xml:"nominalValue,attr"`
	SecurityCategory string          `xml:"securityCategory,attr"`
	SecurityType     string          `xml:"securityType,attr"`
	SecurityName     string          `xml:"securityName,attr"`
	TaxValue         sampleTaxValue  `xml:"taxValue"`
	Payments         []samplePayment `xml:"payment"`
	Stocks           []sampleStock   `xml:"stock"`
}

type sampleTaxValue struct {
	ReferenceDate   string `xml:"referenceDate,attr"`
	QuotationType   string `xml:"quotationType,attr"`
	Quantity        string `xml:"quantity,attr"`
	BalanceCurrency string `xml:"balanceCurrency,attr"`
	UnitPrice       string `xml:"unitPrice,attr"`
	Value           string `xml:"value,attr"`
	ExchangeRate    string `xml:"exchangeRate,attr"`
}

type samplePayment struct {
	PaymentDate                 string `xml:"paymentDate,attr"`
	Name                        string `xml:"name,attr"`
	QuotationType               string `xml:"quotationType,attr"`
	Quantity                    string `xml:"quantity,attr"`
	AmountCurrency              string `xml:"amountCurrency,attr"`
	Amount                      string `xml:"amount,attr"`
	ExchangeRate                string `xml:"exchangeRate,attr"`
	GrossRevenueA               string `xml:"grossRevenueA,attr"`
	GrossRevenueB               string `xml:"grossRevenueB,attr"`
	WithHoldingTaxClaim         string `xml:"withHoldingTaxClaim,attr"`
	AdditionalWithHoldingTaxUSA string `xml:"additionalWithHoldingTaxUSA,attr"`
}

type sampleStock struct {
	ReferenceDate string `xml:"referenceDate,attr"`
	Mutation      string `xml:"mutation,attr"`
}
