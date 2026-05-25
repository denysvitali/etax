package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/etax/internal/barcode"
	"github.com/denysvitali/etax/internal/ech0196"
	"github.com/denysvitali/etax/internal/kursliste"
	"github.com/denysvitali/etax/internal/pdf"
	"github.com/denysvitali/etax/internal/provider"
	"github.com/denysvitali/etax/internal/provider/ibkr"
	"github.com/denysvitali/etax/internal/server"
)

var log = logrus.New()

func main() {
	configureLogger()

	root := newRootCommand()
	if err := root.ExecuteContext(context.Background()); err != nil {
		log.WithError(err).Error("command failed")
		os.Exit(1)
	}
}

func configureLogger() {
	lipgloss.SetColorProfile(termenv.TrueColor)
	log.SetOutput(os.Stderr)
	log.SetFormatter(prettyFormatter{})
	log.SetLevel(logrus.InfoLevel)
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "etax",
		Short:         "Convert broker exports into Swiss eCH-0196 tax statement output",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(
		newConvertCommand(),
		newKurslisteCommand(),
		newPDFCommand(),
		newValidateCommand(),
		newFetchCommand(),
		newServeCommand(),
		newDecodePDF417Command(),
		newProvidersCommand(),
	)
	return cmd
}

func newConvertCommand() *cobra.Command {
	var (
		providerID    string
		input         string
		output        string
		canton        string
		year          int
		firstName     string
		lastName      string
		pdfOut        string
		kurslistePath string
		kurslisteDir  string
		autoKursliste bool
	)

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert a provider export into eCH-0196 XML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" || output == "" || canton == "" || year == 0 {
				return fmt.Errorf("--input, --output, --canton and --year are required")
			}
			p, err := provider.Get(providerID)
			if err != nil {
				return err
			}
			f, err := os.Open(input)
			if err != nil {
				return err
			}
			defer f.Close()

			report, err := p.Parse(cmd.Context(), f)
			if err != nil {
				return err
			}
			options := ech0196.Options{}
			if store, source, err := loadKursliste(cmd.Context(), year, kurslistePath, kurslisteDir, autoKursliste, report.ISINs()); err != nil {
				if strings.TrimSpace(kurslistePath) != "" {
					return err
				}
				log.WithError(err).Warn("Kursliste enrichment unavailable")
			} else if store != nil {
				options.Kursliste = store
				log.WithFields(logrus.Fields{"file": source, "securities": store.Len()}).Info("loaded Kursliste")
			}
			stmt, err := ech0196.FromReportWithOptions(report, canton, year, ech0196.ClientInfo{FirstName: firstName, LastName: lastName}, options)
			if err != nil {
				return err
			}
			xmlData, err := ech0196.WriteFile(stmt, output)
			if err != nil {
				return err
			}
			if pdfOut != "" {
				if err := pdf.WriteSummary(pdfOut, stmt, xmlData); err != nil {
					return err
				}
				log.WithField("file", pdfOut).Info("generated PDF summary")
			}
			log.WithFields(logrus.Fields{
				"file":          output,
				"tax_value_chf": stmt.TotalTaxValue,
				"gross_a_chf":   stmt.TotalGrossRevenueA,
				"gross_b_chf":   stmt.TotalGrossRevenueB,
			}).Info("generated eCH XML")
			return nil
		},
	}
	cmd.Flags().StringVar(&providerID, "provider", "ibkr", "input provider")
	cmd.Flags().StringVar(&input, "input", "", "provider export file")
	cmd.Flags().StringVar(&output, "output", "", "output eCH-0196 XML file")
	cmd.Flags().StringVar(&canton, "canton", "", "Swiss canton code")
	cmd.Flags().IntVar(&year, "year", 0, "tax year")
	cmd.Flags().StringVar(&firstName, "firstname", "", "taxpayer first name")
	cmd.Flags().StringVar(&lastName, "lastname", "", "taxpayer last name")
	cmd.Flags().StringVar(&pdfOut, "pdf", "", "optional PDF summary output")
	cmd.Flags().StringVar(&kurslistePath, "kursliste", "", "optional local Kursliste XML file or directory override")
	cmd.Flags().StringVar(&kurslisteDir, "kursliste-dir", "", "directory for the cached ESTV Kursliste download")
	cmd.Flags().BoolVar(&autoKursliste, "auto-kursliste", true, "automatically download and use the latest ESTV Kursliste")
	return cmd
}

func newKurslisteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kursliste",
		Short: "Manage ESTV ICTax Kursliste files",
	}
	cmd.AddCommand(newKurslisteDownloadCommand())
	return cmd
}

func newKurslisteDownloadCommand() *cobra.Command {
	var (
		year  int
		dir   string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download and cache the latest full ESTV Kursliste XML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if year == 0 {
				return fmt.Errorf("--year is required")
			}
			xmlPath, info, err := kursliste.Ensure(cmd.Context(), year, kursliste.DownloadOptions{
				DestinationDir: dir,
				Force:          force,
			})
			if err != nil {
				return err
			}
			log.WithFields(logrus.Fields{
				"file":        xmlPath,
				"export":      info.ExportTypeShortName,
				"export_date": info.ExportDate,
			}).Info("downloaded Kursliste")
			return nil
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "tax year")
	cmd.Flags().StringVar(&dir, "dir", "", "download directory; defaults to the user cache")
	cmd.Flags().BoolVar(&force, "force", false, "download even when the cached file is current")
	return cmd
}

func loadKursliste(ctx context.Context, year int, path, dir string, auto bool, isins []string) (*kursliste.Store, string, error) {
	if strings.TrimSpace(path) != "" {
		store, err := kursliste.LoadForISINs(path, year, isins)
		return store, path, err
	}
	if !auto {
		return nil, "", nil
	}
	xmlPath, _, err := kursliste.Ensure(ctx, year, kursliste.DownloadOptions{DestinationDir: dir})
	if err != nil {
		return nil, "", err
	}
	store, err := kursliste.LoadForISINs(xmlPath, year, isins)
	return store, xmlPath, err
}

func newDecodePDF417Command() *cobra.Command {
	var (
		output  string
		inflate bool
	)

	cmd := &cobra.Command{
		Use:   "decode-pdf417 [--zlib] [--output payload.xml] barcode.png [...]",
		Short: "Decode one or more PDF417 barcode images",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("at least one image path is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var payload bytes.Buffer
			for _, path := range args {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				results, err := barcode.DecodePDF417Bytes(f)
				_ = f.Close()
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				for _, result := range results {
					payload.Write(result)
				}
			}
			data := payload.Bytes()
			if inflate {
				zr, err := zlib.NewReader(bytes.NewReader(data))
				if err != nil {
					return fmt.Errorf("opening zlib payload: %w", err)
				}
				data, err = io.ReadAll(zr)
				closeErr := zr.Close()
				if err != nil {
					return fmt.Errorf("inflating zlib payload: %w", err)
				}
				if closeErr != nil {
					return fmt.Errorf("closing zlib payload: %w", closeErr)
				}
			}
			if output != "" {
				if err := os.WriteFile(output, data, 0644); err != nil {
					return err
				}
				log.WithField("file", output).Info("wrote decoded PDF417 payload")
				return nil
			}
			_, err := os.Stdout.Write(data)
			return err
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "optional output file")
	cmd.Flags().BoolVar(&inflate, "zlib", false, "zlib-inflate concatenated decoded payload before writing")
	return cmd
}

func newPDFCommand() *cobra.Command {
	var (
		xmlPath string
		output  string
	)

	cmd := &cobra.Command{
		Use:   "pdf",
		Short: "Create a PDF summary from existing eCH XML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if xmlPath == "" || output == "" {
				return fmt.Errorf("--xml and --output are required")
			}
			data, err := os.ReadFile(xmlPath)
			if err != nil {
				return err
			}
			stmt := ech0196.NewTaxStatement("from-xml", "", 0)
			if err := pdf.WriteSummary(output, stmt, data); err != nil {
				return err
			}
			log.WithField("file", output).Info("generated PDF summary")
			return nil
		},
	}
	cmd.Flags().StringVar(&xmlPath, "xml", "", "existing eCH XML file")
	cmd.Flags().StringVar(&output, "output", "", "output PDF file")
	return cmd
}

func newValidateCommand() *cobra.Command {
	var (
		xmlPath string
		schema  string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate eCH XML against an XSD schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			if xmlPath == "" {
				return fmt.Errorf("--xml is required")
			}
			if err := ech0196.ValidateXML(xmlPath, schema); err != nil {
				return err
			}
			log.WithFields(logrus.Fields{"xml": xmlPath, "schema": schema}).Info("validated XML")
			return nil
		},
	}
	cmd.Flags().StringVar(&xmlPath, "xml", "", "eCH XML file")
	cmd.Flags().StringVar(&schema, "schema", "schemas/eCH-0196-2-2.xsd", "XSD schema path")
	return cmd
}

func newFetchCommand() *cobra.Command {
	var (
		providerID string
		token      string
		query      string
		output     string
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch a provider export",
		RunE: func(cmd *cobra.Command, args []string) error {
			if providerID != "ibkr" {
				return fmt.Errorf("fetch is currently implemented for provider ibkr")
			}
			if token == "" || query == "" || output == "" {
				return fmt.Errorf("--token, --query-id and --output are required")
			}
			if err := ibkr.NewFlexClient(token, query).Download(cmd.Context(), output); err != nil {
				return err
			}
			log.WithField("file", output).Info("downloaded provider export")
			return nil
		},
	}
	cmd.Flags().StringVar(&providerID, "provider", "ibkr", "provider")
	cmd.Flags().StringVar(&token, "token", "", "IBKR Flex token")
	cmd.Flags().StringVar(&query, "query-id", "", "IBKR Flex query id")
	cmd.Flags().StringVar(&output, "output", "", "output XML file")
	return cmd
}

func newServeCommand() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the local conversion web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.WithField("url", "http://"+addr).Info("serving")
			return http.ListenAndServe(addr, server.New())
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	return cmd
}

func newProvidersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "List available provider IDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(provider.IDs(), "\n"))
			return err
		},
	}
}

type prettyFormatter struct{}

func (prettyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A94A6"))
	messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E7EDF7"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A94A6"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D7DEE9"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8A8A"))

	levelStyle := styleForLevel(entry.Level)
	line := strings.Builder{}
	line.WriteString(timeStyle.Render(entry.Time.Format("15:04:05")))
	line.WriteString(" ")
	line.WriteString(levelStyle.Render(strings.ToUpper(entry.Level.String())))
	line.WriteString(" ")
	line.WriteString(messageStyle.Render(entry.Message))

	keys := make([]string, 0, len(entry.Data))
	for key := range entry.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := fmt.Sprint(entry.Data[key])
		line.WriteString(" ")
		line.WriteString(keyStyle.Render(key + "="))
		if key == logrus.ErrorKey {
			line.WriteString(errorStyle.Render(value))
			continue
		}
		line.WriteString(valueStyle.Render(value))
	}
	line.WriteString("\n")
	return []byte(line.String()), nil
}

func styleForLevel(level logrus.Level) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true).Width(5).Align(lipgloss.Center)
	switch level {
	case logrus.DebugLevel, logrus.TraceLevel:
		return style.Foreground(lipgloss.Color("#7DD3FC"))
	case logrus.InfoLevel:
		return style.Foreground(lipgloss.Color("#7AE582"))
	case logrus.WarnLevel:
		return style.Foreground(lipgloss.Color("#FFD166"))
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return style.Foreground(lipgloss.Color("#FF5C8A"))
	default:
		return style.Foreground(lipgloss.Color("#E7EDF7"))
	}
}
