package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const detailLabelWidth = 14

type outputStyler struct {
	enabled bool

	profile    termenv.Profile
	lightTheme bool

	headerBase  styleDef
	key         styleDef
	section     styleDef
	dim         styleDef
	warning     styleDef
	error       styleDef
	success     styleDef
	pending     styleDef
	failure     styleDef
	info        styleDef
	identifier  styleDef
	project     styleDef
	metadata    styleDef
	link        styleDef
	placeholder styleDef
}

type styleDef struct {
	light     string
	dark      string
	bold      bool
	italic    bool
	underline bool
}

func newOutputStyler(enabled bool) *outputStyler {
	return newOutputStylerWithProfile(enabled, termenv.TrueColor, false)
}

func newOutputStylerWithProfile(enabled bool, profile termenv.Profile, lightTheme bool) *outputStyler {
	return &outputStyler{
		enabled:    enabled,
		profile:    profile,
		lightTheme: lightTheme,
		headerBase: styleDef{light: "#334155", dark: "#CBD5E1", bold: true},
		key:        styleDef{light: "#1D4ED8", dark: "#7DD3FC", bold: true},
		section:    styleDef{light: "#0F172A", dark: "#E2E8F0", bold: true},
		dim:        styleDef{light: "#6B7280", dark: "#94A3B8"},
		warning:    styleDef{light: "#B45309", dark: "#FCD34D", bold: true},
		error:      styleDef{light: "#B91C1C", dark: "#FCA5A5", bold: true},
		success:    styleDef{light: "#15803D", dark: "#86EFAC", bold: true},
		pending:    styleDef{light: "#7C3AED", dark: "#C4B5FD", bold: true},
		failure:    styleDef{light: "#B91C1C", dark: "#FCA5A5", bold: true},
		info:       styleDef{light: "#0369A1", dark: "#7DD3FC"},
		identifier: styleDef{light: "#4338CA", dark: "#C4B5FD"},
		project:    styleDef{light: "#0F766E", dark: "#99F6E4"},
		metadata:   styleDef{light: "#475569", dark: "#A5B4FC"},
		link:       styleDef{light: "#1D4ED8", dark: "#93C5FD", underline: true},
		placeholder: styleDef{
			light:  "#6B7280",
			dark:   "#64748B",
			italic: true,
		},
	}
}

func newOutputStylerForOptions(opts *Options, w io.Writer, format string) *outputStyler {
	if opts == nil {
		return newOutputStyler(false)
	}
	enabled := shouldColorizeOutput(opts, w, format)
	profile := termenv.Ascii
	if enabled {
		profile = termenv.EnvColorProfile()
		if profile == termenv.Ascii {
			profile = termenv.TrueColor
		}
	}
	output := termenv.NewOutput(w, termenv.WithProfile(profile))
	return newOutputStylerWithProfile(enabled, profile, !output.HasDarkBackground())
}

func shouldColorizeOutput(opts *Options, w io.Writer, format string) bool {
	if opts.NoColor || format == "json" || format == "yaml" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	maxInt := ^uint(0) >> 1
	if fd > uintptr(maxInt) {
		return false
	}
	//nolint:gosec // guarded by the bounds check above; term.IsTerminal requires int
	return term.IsTerminal(int(fd))
}

func (s *outputStyler) apply(style styleDef, text string) string {
	if !s.enabled || text == "" {
		return text
	}
	color := style.dark
	if s.lightTheme {
		color = style.light
	}
	styled := termenv.String(text).Foreground(s.profile.Color(color))
	if style.bold {
		styled = styled.Bold()
	}
	if style.italic {
		styled = styled.Italic()
	}
	if style.underline {
		styled = styled.Underline()
	}
	return styled.String()
}

func (s *outputStyler) Header(column string) string {
	return s.apply(s.styleForColumnHeader(column), column)
}

func (s *outputStyler) Key(label string) string {
	return s.apply(s.key, label)
}

func (s *outputStyler) Section(title string) string {
	return s.apply(s.section, title)
}

func (s *outputStyler) Dim(text string) string {
	return s.apply(s.dim, text)
}

func (s *outputStyler) Warning(text string) string {
	return s.apply(s.warning, text)
}

func (s *outputStyler) Error(text string) string {
	return s.apply(s.error, text)
}

func (s *outputStyler) Placeholder(text string) string {
	return s.apply(s.placeholder, text)
}

// Hyperlink wraps text in an OSC 8 terminal hyperlink escape sequence.
// When styling is disabled or url is empty, it returns text unchanged.
func (s *outputStyler) Hyperlink(text, url string) string {
	if !s.enabled || url == "" || s.profile == termenv.Ascii {
		return text
	}
	return "\x1b]8;;" + url + "\a" + text + "\x1b]8;;\a"
}

func (s *outputStyler) Value(column, value string) string {
	if value == "" {
		return value
	}
	switch normalized := normalizeColumn(column); normalized {
	case "state", "status", "review", "risk", "verdict", "importance", "primary_reason":
		return s.semantic(value)
	case "candidate", "ftbfs", "cancellable":
		return s.boolean(strings.EqualFold(value, "true"))
	case "error":
		if value == "" {
			return value
		}
		return s.Error(value)
	case "url", "link", "dsc url":
		return s.apply(s.link, value)
	case "project", "repo", "name", "package", "tracker", "type", "forge", "target":
		return s.apply(s.project, value)
	case "id", "sha", "bug":
		return s.apply(s.identifier, value)
	case "track", "branch", "suite", "component", "date", "time", "created", "updated", "released", "last updated", "author", "owner", "source", "kind":
		return s.apply(s.metadata, value)
	default:
		if isPlaceholder(value) {
			return s.Placeholder(value)
		}
		return value
	}
}

func (s *outputStyler) DetailValue(label, value string) string {
	if value == "" {
		return value
	}
	switch normalizeColumn(label) {
	case "state", "status", "review", "risk", "verdict", "importance", "primary":
		return s.semantic(value)
	case "candidate", "ftbfs", "cancellable":
		return s.boolean(strings.EqualFold(value, "true"))
	case "error":
		return s.Error(value)
	case "url", "link":
		return s.apply(s.link, value)
	case "project", "package", "tracker", "type", "forge", "team", "maintainer", "owner", "author", "source", "kind":
		return s.apply(s.project, value)
	case "id", "bug":
		return s.apply(s.identifier, value)
	case "created", "updated", "started", "finished", "released", "age (days)", "tracks", "track", "suite", "component":
		return s.apply(s.metadata, value)
	default:
		if isPlaceholder(value) {
			return s.Placeholder(value)
		}
		return value
	}
}

func (s *outputStyler) Action(text string) string {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "create", "add", "set", "synced", "saved", "started", "starting", "stopped", "removed", "cleared", "ok", "done":
		return s.apply(s.success, text)
	case "update", "assign", "syncing", "clearing", "unchanged", "running":
		return s.apply(s.pending, text)
	default:
		return s.apply(s.info, text)
	}
}

func (s *outputStyler) boolean(v bool) string {
	if v {
		return s.apply(s.success, "true")
	}
	return s.apply(s.failure, "false")
}

func (s *outputStyler) semantic(text string) string {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "ok", "stable", "succeeded", "completed", "success", "merged", "authenticated", "done",
		"pass":
		return s.apply(s.success, text)
	case "queued", "running", "building", "candidate", "in progress", "beta", "edge", "started",
		"in-progress":
		return s.apply(s.pending, text)
	case "failed", "error", "cancelled", "canceled", "blocked", "invalid", "interrupted",
		"autopkgtest", "ftbfs", "regression":
		return s.apply(s.failure, text)
	case "warning",
		"no-results", "not-regression", "always-failed", "dependency":
		return s.apply(s.warning, text)
	default:
		return text
	}
}

func (s *outputStyler) styleForColumnHeader(column string) styleDef {
	switch normalizeColumn(column) {
	case "state", "status", "review", "risk", "verdict", "importance":
		return styleDef{light: "#7C2D12", dark: "#FDE68A", bold: true}
	case "project", "repo", "name", "package", "tracker", "type", "forge":
		return styleDef{light: "#0F766E", dark: "#99F6E4", bold: true}
	case "id", "sha", "bug":
		return styleDef{light: "#4338CA", dark: "#C4B5FD", bold: true}
	case "track", "branch", "suite", "component", "date", "time", "created", "updated", "released", "last updated":
		return styleDef{light: "#475569", dark: "#A5B4FC", bold: true}
	default:
		return s.headerBase
	}
}

func renderTableRows(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = lipgloss.Width(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if width := lipgloss.Width(cell); width > widths[i] {
				widths[i] = width
			}
		}
	}

	writeRow := func(row []string) error {
		for i, cell := range row {
			if i > 0 {
				if _, err := io.WriteString(w, "  "); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, padToWidth(cell, widths[i])); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	}

	if err := writeRow(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(row); err != nil {
			return err
		}
	}
	return nil
}

func renderStyledTable(w io.Writer, styler *outputStyler, headers []string, rows [][]string) error {
	styledHeaders := make([]string, len(headers))
	for i, header := range headers {
		styledHeaders[i] = styler.Header(header)
	}

	styledRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		styledRow := make([]string, len(row))
		for i, cell := range row {
			if i < len(headers) {
				styledRow[i] = styler.Value(headers[i], cell)
			} else {
				styledRow[i] = cell
			}
		}
		styledRows = append(styledRows, styledRow)
	}
	return renderTableRows(w, styledHeaders, styledRows)
}

func writeKeyValue(w io.Writer, styler *outputStyler, label, value string) error {
	if value == "" {
		return nil
	}
	padded := fmt.Sprintf("%-*s", detailLabelWidth, label+":")
	_, err := fmt.Fprintf(w, "%s %s\n", styler.Key(padded), styler.DetailValue(label, value))
	return err
}

func writeSectionTitle(w io.Writer, styler *outputStyler, title string) error {
	_, err := fmt.Fprintf(w, "%s\n", styler.Section(title))
	return err
}

func writeWarningLine(w io.Writer, styler *outputStyler, message string) error {
	_, err := fmt.Fprintf(w, "%s %s\n", styler.Warning("warning:"), message)
	return err
}

func writeErrorLine(w io.Writer, styler *outputStyler, message string) error {
	_, err := fmt.Fprintf(w, "%s %s\n", styler.Error("error:"), message)
	return err
}

func padToWidth(text string, width int) string {
	padding := width - lipgloss.Width(text)
	if padding <= 0 {
		return text
	}
	return text + strings.Repeat(" ", padding)
}

func normalizeColumn(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, ":")
	return value
}

func isPlaceholder(value string) bool {
	switch strings.TrimSpace(value) {
	case "-", "—", "(none)", "(not found)", "never", "default":
		return true
	default:
		return false
	}
}
