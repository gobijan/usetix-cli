package output

import (
	"encoding/json"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	clioutput "github.com/basecamp/cli/output"

	"github.com/gobijan/usetix-cli/internal/terminal"
)

type (
	Breadcrumb     = clioutput.Breadcrumb
	Error          = clioutput.Error
	Format         = clioutput.Format
	ResponseOption = clioutput.ResponseOption
)

const (
	FormatAuto   = clioutput.FormatAuto
	FormatJSON   = clioutput.FormatJSON
	FormatStyled = clioutput.FormatStyled
	FormatQuiet  = clioutput.FormatQuiet
	FormatIDs    = clioutput.FormatIDs
	FormatCount  = clioutput.FormatCount
)

var (
	AsError         = clioutput.AsError
	ErrAPI          = clioutput.ErrAPI
	ErrForbidden    = clioutput.ErrForbidden
	ErrNetwork      = clioutput.ErrNetwork
	ErrRateLimit    = clioutput.ErrRateLimit
	ErrUsage        = clioutput.ErrUsage
	ErrUsageHint    = clioutput.ErrUsageHint
	WithBreadcrumbs = clioutput.WithBreadcrumbs
	WithMeta        = clioutput.WithMeta
	WithNotice      = clioutput.WithNotice
	WithSummary     = clioutput.WithSummary
)

type StyledRenderer func(io.Writer) error

type Writer struct {
	format Format
	writer io.Writer
	base   *clioutput.Writer
}

func New(format Format, writer io.Writer) *Writer {
	return &Writer{
		format: format,
		writer: writer,
		base: clioutput.New(clioutput.Options{
			Format: format,
			Writer: writer,
		}),
	}
}

func (writer *Writer) EffectiveFormat() Format {
	return writer.base.EffectiveFormat()
}

func (writer *Writer) OK(data any, styled StyledRenderer, options ...ResponseOption) error {
	if writer.EffectiveFormat() == FormatStyled {
		if styled != nil {
			return styled(writer.writer)
		}
		return writePrettyJSON(writer.writer, data)
	}
	return writer.base.OK(data, options...)
}

func (writer *Writer) Err(err error) error {
	return writer.base.Err(err)
}

func WriteStyledError(destination io.Writer, err error) error {
	structured := AsError(err)
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Red).Render("Error:")
	if _, writeErr := fmt.Fprintf(destination, "%s %s\n", label, terminal.SanitizeLine(structured.Message)); writeErr != nil {
		return writeErr
	}
	if structured.Hint != "" {
		hint := lipgloss.NewStyle().Faint(true).Render(terminal.SanitizeLine(structured.Hint))
		_, writeErr := fmt.Fprintf(destination, "%s\n", hint)
		return writeErr
	}
	return nil
}

func writePrettyJSON(destination io.Writer, data any) error {
	encoder := json.NewEncoder(destination)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
