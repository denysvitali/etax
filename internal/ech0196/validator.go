package ech0196

import (
	"fmt"
	"os"
	"os/exec"
)

func ValidateXML(xmlPath, schemaPath string) error {
	if schemaPath == "" {
		return fmt.Errorf("schema path is required")
	}
	if _, err := os.Stat(schemaPath); err != nil {
		return fmt.Errorf("schema unavailable at %s: %w", schemaPath, err)
	}
	if _, err := exec.LookPath("xmllint"); err != nil {
		return fmt.Errorf("xmllint is required for XSD validation: %w", err)
	}
	cmd := exec.Command("xmllint", "--noout", "--schema", schemaPath, xmlPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("XSD validation failed: %w\n%s", err, string(out))
	}
	return nil
}
