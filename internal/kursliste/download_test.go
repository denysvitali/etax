package kursliste

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestExportSelectsNewestFullExport(t *testing.T) {
	client := fakeHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/xmls.json" {
			return response(404, ""), nil
		}
		return response(200, `{
			"status": "SUCCESS",
			"error": null,
			"data": [
				{"complete": true, "exportDate": 3000, "exportFile": {"id": 3, "fileName": "kursliste_2025.zip", "fileSize": 30, "fileHash": "hash-3"}, "exportType": {"shortName": "THIRD.INIT.220"}},
				{"complete": true, "exportDate": 4000, "exportFile": {"id": 4, "fileName": "kursliste_2025.zip", "fileSize": 10, "fileHash": "hash-4"}, "exportType": {"shortName": "THIRD.DELTA.220"}},
				{"complete": true, "exportDate": 2000, "exportFile": {"id": 2, "fileName": "kursliste_2025.zip", "fileSize": 20, "fileHash": "hash-2"}, "exportType": {"shortName": "THIRD.INIT.220"}}
			]
		}`), nil
	})

	info, err := LatestExport(context.Background(), 2025, DownloadOptions{APIURL: "https://ictax.test/xmls.json", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	if info.FileID != 3 || info.FileHash != "hash-3" || info.ExportTypeShortName != "THIRD.INIT.220" {
		t.Fatalf("selected export = %+v", info)
	}
}

func TestEnsureDownloadsExtractsAndCachesKursliste(t *testing.T) {
	zipData := testKurslisteZip(t)
	var downloads int

	client := fakeHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/session.json":
			return response(200, `{"data":{"csrfToken":"token"}}`), nil
		case r.URL.Path == "/xmls.json":
			if r.Header.Get("X-CSRF-TOKEN") != "token" {
				t.Errorf("missing CSRF token")
			}
			return response(200, fmt.Sprintf(`{
				"status": "SUCCESS",
				"error": null,
				"data": [
					{"complete": true, "exportDate": 3000, "exportFile": {"id": 42, "fileName": "kursliste_2025.zip", "fileSize": %d, "fileHash": %q}, "exportType": {"shortName": "THIRD.INIT.220"}}
				]
			}`, len(zipData), "download-token")), nil
		case r.URL.Path == "/download/42/download-token/kursliste_2025.zip":
			downloads++
			return bytesResponse(200, zipData), nil
		default:
			return response(404, ""), nil
		}
	})

	dir := t.TempDir()
	options := DownloadOptions{
		DestinationDir: dir,
		APIURL:         "https://ictax.test/xmls.json",
		SessionURL:     "https://ictax.test/session.json",
		DownloadBase:   "https://ictax.test/download",
		Client:         client,
	}
	xmlPath, info, err := Ensure(context.Background(), 2025, options)
	if err != nil {
		t.Fatal(err)
	}
	if info.FileID != 42 {
		t.Fatalf("FileID = %d, want 42", info.FileID)
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d, want 1", downloads)
	}
	if xmlPath != filepath.Join(dir, "kursliste_2025.xml") {
		t.Fatalf("xmlPath = %q", xmlPath)
	}
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `isin="US0378331005"`) {
		t.Fatalf("extracted XML = %s", data)
	}

	_, _, err = Ensure(context.Background(), 2025, options)
	if err != nil {
		t.Fatal(err)
	}
	if downloads != 1 {
		t.Fatalf("downloads after cache hit = %d, want 1", downloads)
	}
}

func testKurslisteZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("nested/kursliste_2025.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`<kursliste year="2025"><share valorNumber="37833100" isin="US0378331005"/></kursliste>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func fakeHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func response(status int, body string) *http.Response {
	return bytesResponse(status, []byte(body))
}

func bytesResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}
