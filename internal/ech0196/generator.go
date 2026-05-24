package ech0196

import (
	"encoding/xml"
	"fmt"
	"os"
)

func Marshal(stmt TaxStatement) ([]byte, error) {
	body, err := xml.MarshalIndent(stmt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling eCH-0196 XML: %w", err)
	}
	out := append([]byte(xml.Header), body...)
	out = append(out, '\n')
	return out, nil
}

func WriteFile(stmt TaxStatement, path string) ([]byte, error) {
	data, err := Marshal(stmt)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("writing eCH-0196 XML: %w", err)
	}
	return data, nil
}
