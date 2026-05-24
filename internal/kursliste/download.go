package kursliste

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultAPIURL      = "https://www.ictax.admin.ch/extern/api/xml/xmls.json"
	defaultSessionURL  = "https://www.ictax.admin.ch/extern/api/authentication/session.json"
	defaultDownloadURL = "https://www.ictax.admin.ch/extern/api/download"
)

type DownloadOptions struct {
	DestinationDir string
	Force          bool
	Client         *http.Client
	APIURL         string
	SessionURL     string
	DownloadBase   string
}

type ExportInfo struct {
	FileID              int    `json:"file_id"`
	FileHash            string `json:"file_hash"`
	FileName            string `json:"file_name"`
	FileSize            int64  `json:"file_size"`
	ExportDate          int64  `json:"export_date"`
	ExportTypeShortName string `json:"export_type_short_name"`
}

type cacheMetadata struct {
	ExportInfo
	DownloadedAt string `json:"downloaded_at"`
}

func Ensure(ctx context.Context, year int, options DownloadOptions) (string, ExportInfo, error) {
	if year == 0 {
		return "", ExportInfo{}, fmt.Errorf("year is required")
	}
	dir, err := destinationDir(options.DestinationDir)
	if err != nil {
		return "", ExportInfo{}, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", ExportInfo{}, err
	}

	xmlPath := filepath.Join(dir, fmt.Sprintf("kursliste_%d.xml", year))
	metaPath := filepath.Join(dir, fmt.Sprintf("kursliste_%d.meta.json", year))
	client := httpClient(options.Client)
	_ = initializeSession(ctx, client, endpoint(options.SessionURL, defaultSessionURL))
	options.Client = client

	info, err := LatestExport(ctx, year, options)
	if err != nil {
		if _, statErr := os.Stat(xmlPath); statErr == nil && !options.Force {
			return xmlPath, ExportInfo{}, nil
		}
		return "", ExportInfo{}, err
	}
	if !options.Force && cacheMatches(metaPath, info) {
		if _, err := os.Stat(xmlPath); err == nil {
			return xmlPath, info, nil
		}
	}

	zipPath, err := downloadZip(ctx, client, downloadURL(endpoint(options.DownloadBase, defaultDownloadURL), info), dir, info)
	if err != nil {
		return "", ExportInfo{}, err
	}
	defer os.Remove(zipPath)

	if err := extractKurslisteXML(zipPath, xmlPath, year); err != nil {
		return "", ExportInfo{}, err
	}
	if err := writeMetadata(metaPath, info); err != nil {
		return "", ExportInfo{}, err
	}
	return xmlPath, info, nil
}

func DefaultDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "etax", "kursliste"), nil
}

func LatestExport(ctx context.Context, year int, options DownloadOptions) (ExportInfo, error) {
	client := httpClient(options.Client)
	body, err := json.Marshal(map[string]any{
		"from": 0,
		"size": 100,
		"sort": []any{},
		"year": year,
		"xsdType": map[string]string{
			"categoryShortName": "XSDTYP",
			"shortName":         "kursliste-2.2.0",
		},
	})
	if err != nil {
		return ExportInfo{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(options.APIURL, defaultAPIURL), bytes.NewReader(body))
	if err != nil {
		return ExportInfo{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return ExportInfo{}, fmt.Errorf("querying ICTax Kursliste metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExportInfo{}, fmt.Errorf("querying ICTax Kursliste metadata: HTTP %s", resp.Status)
	}

	var decoded struct {
		Status string `json:"status"`
		Error  any    `json:"error"`
		Data   []struct {
			Complete   bool  `json:"complete"`
			ExportDate int64 `json:"exportDate"`
			ExportFile struct {
				ID       int    `json:"id"`
				FileName string `json:"fileName"`
				FileSize int64  `json:"fileSize"`
				FileHash string `json:"fileHash"`
			} `json:"exportFile"`
			ExportType struct {
				ShortName string `json:"shortName"`
			} `json:"exportType"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ExportInfo{}, fmt.Errorf("decoding ICTax Kursliste metadata: %w", err)
	}
	if decoded.Status != "SUCCESS" {
		return ExportInfo{}, fmt.Errorf("ICTax Kursliste metadata status %q: %v", decoded.Status, decoded.Error)
	}

	var candidates []ExportInfo
	for _, item := range decoded.Data {
		if !item.Complete || !strings.HasPrefix(item.ExportType.ShortName, "THIRD.INIT.") {
			continue
		}
		if item.ExportFile.ID == 0 || item.ExportFile.FileHash == "" || item.ExportFile.FileName == "" {
			continue
		}
		candidates = append(candidates, ExportInfo{
			FileID:              item.ExportFile.ID,
			FileHash:            item.ExportFile.FileHash,
			FileName:            item.ExportFile.FileName,
			FileSize:            item.ExportFile.FileSize,
			ExportDate:          item.ExportDate,
			ExportTypeShortName: item.ExportType.ShortName,
		})
	}
	if len(candidates) == 0 {
		return ExportInfo{}, fmt.Errorf("no full Kursliste export found for year %d", year)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ExportDate > candidates[j].ExportDate
	})
	return candidates[0], nil
}

func initializeSession(ctx context.Context, client *http.Client, sessionURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var decoded struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}
	if decoded.Data.CSRFToken != "" {
		client.Transport = csrfTransport{base: client.Transport, token: decoded.Data.CSRFToken}
	}
	return nil
}

func downloadZip(ctx context.Context, client *http.Client, url, dir string, info ExportInfo) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading ICTax Kursliste: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("downloading ICTax Kursliste: HTTP %s", resp.Status)
	}

	tmp, err := os.CreateTemp(dir, "kursliste-*.zip")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	_, copyErr := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	return path, nil
}

func extractKurslisteXML(zipPath, xmlPath string, year int) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	wanted := fmt.Sprintf("kursliste_%d.xml", year)
	var candidate *zip.File
	for _, f := range r.File {
		if filepath.Base(f.Name) == wanted {
			candidate = f
			break
		}
		if candidate == nil && strings.EqualFold(filepath.Ext(f.Name), ".xml") {
			candidate = f
		}
	}
	if candidate == nil {
		return fmt.Errorf("downloaded Kursliste ZIP contains no XML file")
	}

	source, err := candidate.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	tmp, err := os.CreateTemp(filepath.Dir(xmlPath), filepath.Base(xmlPath)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, copyErr := io.Copy(tmp, source)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	if err := os.Rename(tmpPath, xmlPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func writeMetadata(path string, info ExportInfo) error {
	data, err := json.MarshalIndent(cacheMetadata{
		ExportInfo:   info,
		DownloadedAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func cacheMatches(path string, info ExportInfo) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var meta cacheMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return false
	}
	return meta.FileID == info.FileID && strings.EqualFold(meta.FileHash, info.FileHash)
}

func destinationDir(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		return path, nil
	}
	return DefaultDir()
}

func downloadURL(base string, info ExportInfo) string {
	return strings.TrimRight(base, "/") + fmt.Sprintf("/%d/%s/%s", info.FileID, info.FileHash, info.FileName)
}

func endpoint(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	jar, _ := cookiejar.New(nil)
	return &http.Client{Timeout: 2 * time.Minute, Jar: jar}
}

type csrfTransport struct {
	base  http.RoundTripper
	token string
}

func (t csrfTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("X-CSRF-TOKEN", t.token)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
