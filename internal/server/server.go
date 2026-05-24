package server

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"etax/internal/domain"
	"etax/internal/ech0196"
	"etax/internal/kursliste"
	"etax/internal/pdf"
	"etax/internal/provider"
)

func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/convert", convert)
	return mux
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	_ = page.Execute(w, provider.IDs())
}

func convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("input")
	if err != nil {
		http.Error(w, "input file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	p, err := provider.Get(r.FormValue("provider"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil {
		http.Error(w, "year is required", http.StatusBadRequest)
		return
	}
	report, err := p.Parse(context.Background(), file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	options := ech0196.Options{}
	if path := strings.TrimSpace(r.FormValue("kursliste")); path != "" {
		store, err := kursliste.Load(path, year)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		options.Kursliste = store
	} else if store, err := autoKursliste(r.Context(), year, reportISINs(report)); err == nil {
		options.Kursliste = store
	}
	stmt, err := ech0196.FromReportWithOptions(report, r.FormValue("canton"), year, ech0196.ClientInfo{
		FirstName: r.FormValue("firstname"),
		LastName:  r.FormValue("lastname"),
	}, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	xmlData, err := ech0196.Marshal(stmt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.FormValue("format") == "pdf" {
		pdfData, err := pdf.Bytes(stmt, xmlData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="etax-%d.pdf"`, year))
		_, _ = w.Write(pdfData)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="etax-%d.xml"`, year))
	_, _ = w.Write(xmlData)
}

var page = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>eTax Converter</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 0; background: #f7f8fa; color: #19212a; }
    main { max-width: 760px; margin: 48px auto; padding: 0 24px; }
    form { background: white; border: 1px solid #d8dde4; border-radius: 8px; padding: 24px; display: grid; gap: 16px; }
    label { display: grid; gap: 6px; font-size: 14px; font-weight: 600; }
    input, select, button { font: inherit; padding: 10px 12px; border: 1px solid #b8c0cc; border-radius: 6px; }
    button { background: #14532d; color: white; border-color: #14532d; cursor: pointer; }
    .row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
    .label-title { align-items: center; display: flex; gap: 8px; }
    .help { position: relative; display: inline-flex; }
    .help button { align-items: center; background: #eef2f7; border-color: #b8c0cc; border-radius: 999px; color: #243447; display: inline-flex; font-size: 13px; font-weight: 700; height: 22px; justify-content: center; line-height: 1; padding: 0; width: 22px; }
    .help-panel { background: #fff; border: 1px solid #c9d1dc; border-radius: 8px; box-shadow: 0 12px 28px rgba(25, 33, 42, 0.16); display: none; font-size: 13px; font-weight: 400; left: 0; line-height: 1.45; max-width: min(560px, calc(100vw - 64px)); padding: 14px 16px; position: absolute; top: 30px; width: 520px; z-index: 10; }
    .help-panel[data-active="true"], .help:focus-within .help-panel, .help:hover .help-panel { display: block; }
    .help-panel p { margin: 0 0 10px; }
    .help-panel ol { margin: 0; padding-left: 18px; }
    .help-panel li { margin: 6px 0; }
    .help-panel a { color: #14532d; font-weight: 600; }
    @media (max-width: 640px) { .row { grid-template-columns: 1fr; } main { margin: 24px auto; } }
  </style>
</head>
<body>
<main>
  <h1>eTax Converter</h1>
  <form action="/convert" method="post" enctype="multipart/form-data">
    <label>Provider
      <select name="provider" id="provider">{{range .}}<option value="{{.}}">{{.}}</option>{{end}}</select>
    </label>
    <label>
      <span class="label-title">Export file
        <span class="help" data-provider-help="ibkr">
          <button type="button" aria-controls="ibkr-export-help" aria-expanded="false" title="IBKR export help">?</button>
          <span class="help-panel" id="ibkr-export-help" role="tooltip">
            <p>Provide an Interactive Brokers Activity Flex Query XML file for the full tax year.</p>
            <ol>
              <li>In IBKR Client Portal, open <strong>Reports &gt; Flex Queries</strong>.</li>
              <li>Create or edit an <strong>Activity Flex Query</strong> named <strong>etax</strong>.</li>
              <li>Select these sections and use <strong>Select All</strong> fields in each: <strong>Financial Instrument Information</strong>, <strong>Open Positions</strong> with <strong>Summary</strong>, <strong>Trades</strong> with <strong>Execution</strong>, and <strong>Cash Transactions</strong> with dividend, withholding tax, interest, fees, deposits and withdrawals, and detail options.</li>
              <li>Delivery Configuration: choose your account, set <strong>Format</strong> to <strong>XML</strong>, and set <strong>Period</strong> to the full tax year. Use Annual or a custom range from Jan 1 to Dec 31 for a completed return; Year to Date is only for the current tax year.</li>
              <li>General Configuration: use <strong>Date Format yyyyMMdd</strong>, <strong>Time Format HHmmss</strong>, <strong>Include Currency Rates: Yes</strong>, <strong>Display Account Alias: No</strong>, and <strong>Breakout by Day: No</strong>.</li>
              <li>Run the saved Flex Query and upload the downloaded <code>.xml</code> file here. The file should contain one <code>FlexQueryResponse</code> with a <code>FlexStatement</code>.</li>
            </ol>
            <p>IBKR docs: <a href="https://www.ibkrguides.com/complianceportal/activityflexqueries.htm" target="_blank" rel="noreferrer">create</a> and <a href="https://www.ibkrguides.com/complianceportal/runaflexquery.htm" target="_blank" rel="noreferrer">run</a> Flex Queries.</p>
          </span>
        </span>
      </span>
      <input type="file" name="input" accept=".xml,text/xml,application/xml" required>
    </label>
    <div class="row">
      <label>Canton <input name="canton" value="ZH" required></label>
      <label>Tax year <input name="year" type="number" value="2025" required></label>
    </div>
    <div class="row">
      <label>First name <input name="firstname"></label>
      <label>Last name <input name="lastname"></label>
    </div>
    <label>Format
      <select name="format"><option value="xml">eCH XML</option><option value="pdf">PDF summary</option></select>
    </label>
    <button type="submit">Convert</button>
  </form>
</main>
<script>
  const provider = document.querySelector('#provider');
  const ibkrHelp = document.querySelector('[data-provider-help="ibkr"]');
  const helpButton = ibkrHelp.querySelector('button');
  const helpPanel = ibkrHelp.querySelector('.help-panel');

  function syncProviderHelp() {
    ibkrHelp.hidden = provider.value !== 'ibkr';
  }

  provider.addEventListener('change', syncProviderHelp);
  helpButton.addEventListener('click', () => {
    const active = helpPanel.dataset.active !== 'true';
    helpPanel.dataset.active = String(active);
    helpButton.setAttribute('aria-expanded', String(active));
  });
  document.addEventListener('click', (event) => {
    if (!ibkrHelp.contains(event.target)) {
      helpPanel.dataset.active = 'false';
      helpButton.setAttribute('aria-expanded', 'false');
    }
  });
  syncProviderHelp();
</script>
</body>
</html>`))

func autoKursliste(ctx context.Context, year int, isins []string) (*kursliste.Store, error) {
	xmlPath, _, err := kursliste.Ensure(ctx, year, kursliste.DownloadOptions{})
	if err != nil {
		return nil, err
	}
	return kursliste.LoadForISINs(xmlPath, year, isins)
}

func reportISINs(report *domain.Report) []string {
	if report == nil {
		return nil
	}
	seen := map[string]bool{}
	var isins []string
	for _, position := range report.Positions {
		if position.ISIN != "" && !seen[position.ISIN] {
			seen[position.ISIN] = true
			isins = append(isins, position.ISIN)
		}
	}
	for _, trade := range report.Trades {
		if trade.ISIN != "" && !seen[trade.ISIN] {
			seen[trade.ISIN] = true
			isins = append(isins, trade.ISIN)
		}
	}
	for _, cashflow := range report.CashFlows {
		if cashflow.ISIN != "" && !seen[cashflow.ISIN] {
			seen[cashflow.ISIN] = true
			isins = append(isins, cashflow.ISIN)
		}
	}
	return isins
}
