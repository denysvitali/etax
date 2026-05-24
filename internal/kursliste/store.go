package kursliste

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var errYearMismatch = errors.New("kursliste year mismatch")

var securityElements = map[string]bool{
	"bond":         true,
	"coinBullion":  true,
	"currencyNote": true,
	"derivative":   true,
	"fund":         true,
	"liborSwap":    true,
	"share":        true,
}

type Security struct {
	ValorNumber   string
	ISIN          string
	SecurityGroup string
	SecurityType  string
	SecurityName  string
	Country       string
	Currency      string
	NominalValue  string
}

type Store struct {
	byISIN map[string]Security
}

func NewStore() *Store {
	return &Store{byISIN: map[string]Security{}}
}

func Load(path string, year int) (*Store, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	store := NewStore()
	if !info.IsDir() {
		if err := store.loadFile(path, year); err != nil {
			return nil, err
		}
		return store, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".xml" {
			continue
		}
		files = append(files, filepath.Join(path, entry.Name()))
	}
	sort.Strings(files)

	var lastErr error
	for _, file := range files {
		err := store.loadFile(file, year)
		switch {
		case err == nil:
		case errors.Is(err, errYearMismatch):
			lastErr = err
		default:
			return nil, err
		}
	}
	if store.Len() == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("no Kursliste XML for year %d in %s", year, path)
		}
		return nil, fmt.Errorf("no Kursliste securities found in %s", path)
	}
	return store, nil
}

func LoadForISINs(path string, year int, isins []string) (*Store, error) {
	targets := targetISINs(isins)
	if len(targets) == 0 {
		return NewStore(), nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	store := NewStore()
	if !info.IsDir() {
		if err := store.loadFileForISINs(path, year, targets); err != nil {
			return nil, err
		}
		return store, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".xml" {
			continue
		}
		files = append(files, filepath.Join(path, entry.Name()))
	}
	sort.Strings(files)

	var lastErr error
	for _, file := range files {
		err := store.loadFileForISINs(file, year, targets)
		switch {
		case err == nil:
			if store.Len() == len(targets) {
				return store, nil
			}
		case errors.Is(err, errYearMismatch):
			lastErr = err
		default:
			return nil, err
		}
	}
	if store.Len() == 0 && lastErr != nil {
		return nil, fmt.Errorf("no Kursliste XML for year %d in %s", year, path)
	}
	return store, nil
}

func (s *Store) LookupISIN(isin string) (Security, bool) {
	if s == nil {
		return Security{}, false
	}
	sec, ok := s.byISIN[normalizeISIN(isin)]
	return sec, ok
}

func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	return len(s.byISIN)
}

func (s *Store) loadFile(path string, year int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := s.LoadReader(f, year); err != nil {
		if errors.Is(err, errYearMismatch) {
			return err
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (s *Store) loadFileForISINs(path string, year int, targets map[string]bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := s.LoadReaderForISINs(f, year, targets); err != nil {
		if errors.Is(err, errYearMismatch) {
			return err
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (s *Store) LoadReader(r io.Reader, year int) error {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charsetReader
	seenRoot := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !seenRoot {
			seenRoot = true
			if start.Name.Local != "kursliste" {
				return fmt.Errorf("expected Kursliste root element, got %q", start.Name.Local)
			}
			if rootYear := attr(start, "year"); year != 0 && rootYear != "" {
				parsed, err := strconv.Atoi(rootYear)
				if err != nil {
					return fmt.Errorf("invalid Kursliste year %q", rootYear)
				}
				if parsed != year {
					return errYearMismatch
				}
			}
		}
		if !securityElements[start.Name.Local] {
			continue
		}
		sec := securityFromStart(start)
		if sec.ISIN != "" && sec.ValorNumber != "" {
			s.byISIN[sec.ISIN] = sec
		}
		if err := dec.Skip(); err != nil {
			return err
		}
	}
}

func (s *Store) LoadReaderForISINs(r io.Reader, year int, targets map[string]bool) error {
	reader := bufio.NewReader(r)
	seenRoot := false
	latin1 := false
	for {
		lineBytes, err := reader.ReadBytes('\n')
		if len(lineBytes) > 0 {
			line := string(lineBytes)
			if !seenRoot && strings.Contains(strings.ToLower(line), `encoding="iso-8859-1"`) {
				latin1 = true
			}
			if latin1 {
				line = latin1String(lineBytes)
			}
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "<?xml") || !strings.HasPrefix(line, "<") || strings.HasPrefix(line, "</") {
				if err != nil {
					return nil
				}
				continue
			}
			name := elementNameFromLine(line)
			if name != "" && !seenRoot {
				seenRoot = true
				if name != "kursliste" {
					return fmt.Errorf("expected Kursliste root element, got %q", name)
				}
				if rootYear := attrFromLine(line, "year"); year != 0 && rootYear != "" {
					parsed, err := strconv.Atoi(rootYear)
					if err != nil {
						return fmt.Errorf("invalid Kursliste year %q", rootYear)
					}
					if parsed != year {
						return errYearMismatch
					}
				}
			}
			if securityElements[name] {
				isin := normalizeISIN(attrFromLine(line, "isin"))
				if targets[isin] {
					sec := securityFromLine(line)
					if sec.ISIN != "" && sec.ValorNumber != "" {
						s.byISIN[sec.ISIN] = sec
					}
					if s.Len() == len(targets) {
						return nil
					}
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func securityFromLine(line string) Security {
	return Security{
		ValorNumber:   normalizeValor(attrFromLine(line, "valorNumber")),
		ISIN:          normalizeISIN(attrFromLine(line, "isin")),
		SecurityGroup: strings.TrimSpace(attrFromLine(line, "securityGroup")),
		SecurityType:  strings.TrimSpace(attrFromLine(line, "securityType")),
		SecurityName:  firstNonEmpty(attrFromLine(line, "securityName"), attrFromLine(line, "institutionName")),
		Country:       strings.TrimSpace(attrFromLine(line, "country")),
		Currency:      strings.TrimSpace(attrFromLine(line, "currency")),
		NominalValue:  strings.TrimSpace(attrFromLine(line, "nominalValue")),
	}
}

func securityFromStart(start xml.StartElement) Security {
	return Security{
		ValorNumber:   normalizeValor(attr(start, "valorNumber")),
		ISIN:          normalizeISIN(attr(start, "isin")),
		SecurityGroup: strings.TrimSpace(attr(start, "securityGroup")),
		SecurityType:  strings.TrimSpace(attr(start, "securityType")),
		SecurityName:  firstNonEmpty(attr(start, "securityName"), attr(start, "institutionName")),
		Country:       strings.TrimSpace(attr(start, "country")),
		Currency:      strings.TrimSpace(attr(start, "currency")),
		NominalValue:  strings.TrimSpace(attr(start, "nominalValue")),
	}
}

func elementNameFromLine(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "<") || strings.HasPrefix(line, "</") || strings.HasPrefix(line, "<?") || strings.HasPrefix(line, "<!") {
		return ""
	}
	line = strings.TrimPrefix(line, "<")
	end := strings.IndexAny(line, " \t\r\n>/")
	if end == -1 {
		return ""
	}
	name := line[:end]
	if colon := strings.LastIndex(name, ":"); colon >= 0 {
		name = name[colon+1:]
	}
	return name
}

func attrFromLine(line, name string) string {
	for _, quote := range []byte{'"', '\''} {
		pattern := name + "=" + string(quote)
		start := strings.Index(line, pattern)
		if start < 0 {
			continue
		}
		start += len(pattern)
		end := strings.IndexByte(line[start:], quote)
		if end < 0 {
			return ""
		}
		return html.UnescapeString(line[start : start+end])
	}
	return ""
}

func startElementFromLine(line string) (xml.StartElement, bool, error) {
	dec := xml.NewDecoder(strings.NewReader(line))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return xml.StartElement{}, false, nil
		}
		if err != nil {
			return xml.StartElement{}, false, err
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			return start, true, nil
		}
	}
}

func attr(start xml.StartElement, name string) string {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func targetISINs(isins []string) map[string]bool {
	targets := map[string]bool{}
	for _, isin := range isins {
		if normalized := normalizeISIN(isin); normalized != "" {
			targets[normalized] = true
		}
	}
	return targets
}

func normalizeISIN(isin string) string {
	isin = strings.ToUpper(strings.TrimSpace(isin))
	if len(isin) != 12 {
		return ""
	}
	return isin
}

func normalizeValor(valor string) string {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return ""
	}
	n, err := strconv.ParseInt(valor, 10, 64)
	if err != nil || n < 100 || n > 999999999999 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.ReplaceAll(charset, "_", "-")) {
	case "iso-8859-1", "latin-1", "latin1":
		return &latin1Reader{r: input}, nil
	default:
		return nil, fmt.Errorf("unsupported XML charset %q", charset)
	}
}

type latin1Reader struct {
	r       io.Reader
	pending []byte
	buf     [4096]byte
}

func (r *latin1Reader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if len(r.pending) > 0 {
			n := copy(p[written:], r.pending)
			written += n
			r.pending = r.pending[n:]
			continue
		}

		n, err := r.r.Read(r.buf[:])
		if n > 0 {
			r.pending = r.pending[:0]
			for _, b := range r.buf[:n] {
				if b < utf8.RuneSelf {
					r.pending = append(r.pending, b)
					continue
				}
				var encoded [utf8.UTFMax]byte
				size := utf8.EncodeRune(encoded[:], rune(b))
				r.pending = append(r.pending, encoded[:size]...)
			}
			continue
		}
		if err != nil {
			if written > 0 {
				return written, nil
			}
			return 0, err
		}
	}
	return written, nil
}

func latin1String(data []byte) string {
	var out []byte
	for _, b := range data {
		if b < utf8.RuneSelf {
			out = append(out, b)
			continue
		}
		var encoded [utf8.UTFMax]byte
		size := utf8.EncodeRune(encoded[:], rune(b))
		out = append(out, encoded[:size]...)
	}
	return string(out)
}
