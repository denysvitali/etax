package ibkr

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	flexRequestURL   = "https://flex.interactivebrokers.com/FlexWebService/SendRequest"
	flexStatementURL = "https://flex.interactivebrokers.com/FlexWebService/GetStatement"
)

type FlexClient struct {
	Token  string
	Query  string
	Client *http.Client
}

func NewFlexClient(token, query string) *FlexClient {
	return &FlexClient{
		Token:  token,
		Query:  query,
		Client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *FlexClient) Download(ctx context.Context, outputPath string) error {
	if c.Token == "" || c.Query == "" {
		return fmt.Errorf("token and query id are required")
	}
	ref, err := c.requestReference(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		done, err := c.downloadStatement(ctx, ref, outputPath)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for IBKR Flex statement")
}

func (c *FlexClient) requestReference(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s?t=%s&q=%s&v=3", flexRequestURL, c.Token, c.Query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting Flex reference: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed struct {
		ReferenceCode string `xml:"ReferenceCode"`
		Status        string `xml:"Status"`
		ErrorCode     string `xml:"ErrorCode"`
		ErrorMessage  string `xml:"ErrorMessage"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing Flex response: %w", err)
	}
	if parsed.ReferenceCode == "" {
		return "", fmt.Errorf("IBKR Flex request failed: status=%s code=%s message=%s", parsed.Status, parsed.ErrorCode, parsed.ErrorMessage)
	}
	return parsed.ReferenceCode, nil
}

func (c *FlexClient) downloadStatement(ctx context.Context, referenceCode, outputPath string) (bool, error) {
	url := fmt.Sprintf("%s?t=%s&q=%s&v=3", flexStatementURL, c.Token, referenceCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf("downloading Flex statement: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("IBKR returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	if looksLikePending(body) {
		return false, nil
	}
	return true, os.WriteFile(outputPath, body, 0644)
}

func looksLikePending(body []byte) bool {
	var parsed struct {
		Status string `xml:"Status"`
	}
	if xml.Unmarshal(body, &parsed) != nil {
		return false
	}
	return parsed.Status != ""
}
