package server

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"etax/internal/ech0196"
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
	stmt, err := ech0196.FromReport(report, r.FormValue("canton"), year, ech0196.ClientInfo{
		FirstName: r.FormValue("firstname"),
		LastName:  r.FormValue("lastname"),
	})
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
    @media (max-width: 640px) { .row { grid-template-columns: 1fr; } main { margin: 24px auto; } }
  </style>
</head>
<body>
<main>
  <h1>eTax Converter</h1>
  <form action="/convert" method="post" enctype="multipart/form-data">
    <label>Provider
      <select name="provider">{{range .}}<option value="{{.}}">{{.}}</option>{{end}}</select>
    </label>
    <label>Export file <input type="file" name="input" required></label>
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
</body>
</html>`))
