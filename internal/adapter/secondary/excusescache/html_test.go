// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package excusescache

import (
	"strings"
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "anchor tags stripped",
			input: `<a href="https://example.com">click here</a>`,
			want:  "click here",
		},
		{
			name:  "span with style stripped",
			input: `<span style="background:#87d96c">Pass</span>`,
			want:  "Pass",
		},
		{
			name:  "nested tags stripped",
			input: `<a href="url"><span style="color:red">text</span></a>`,
			want:  "text",
		},
		{
			name:  "entities decoded",
			input: "foo &amp; bar &lt;baz&gt;",
			want:  "foo & bar <baz>",
		},
		{
			name:  "control characters stripped",
			input: "hello\x1b[31mworld\x1b[0m",
			want:  "hello[31mworld[0m",
		},
		{
			name:  "unicode directional overrides stripped",
			input: "hello\u202Aworld\u202E",
			want:  "helloworld",
		},
		{
			name:  "tabs and newlines preserved",
			input: "line1\nline2\tindented",
			want:  "line1\nline2\tindented",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "build failure HTML",
			input: `missing build on <a href="https://launchpad.net/ubuntu/+source/buildbox/1.4.0-1/+latestbuild/armhf" target="_blank">armhf</a>: buildbox`,
			want:  "missing build on armhf: buildbox",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAutopkgtestLine(t *testing.T) {
	t.Run("non-autopkgtest line returns nil", func(t *testing.T) {
		got := parseAutopkgtestLine("missing build on armhf")
		if got != nil {
			t.Fatalf("expected nil, got %d entries", len(got))
		}
	})

	t.Run("multi-arch line extracts all entries", func(t *testing.T) {
		line := `autopkgtest for alembic/1.18.4-1: ` +
			`<a href="https://autopkgtest.ubuntu.com/packages/a/alembic/resolute/amd64">amd64</a>: ` +
			`<a href="https://autopkgtest.ubuntu.com/running"><span style="background:#99ddff">Test in progress</span></a>, ` +
			`<a href="https://autopkgtest.ubuntu.com/packages/a/alembic/resolute/arm64">arm64</a>: ` +
			`<a href="https://objectstorage.example.com/log.gz"><span style="background:#87d96c">Pass</span></a>`

		results := parseAutopkgtestLine(line)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		if results[0].Package != "alembic/1.18.4-1" {
			t.Errorf("results[0].Package = %q, want %q", results[0].Package, "alembic/1.18.4-1")
		}
		if results[0].Architecture != "amd64" {
			t.Errorf("results[0].Architecture = %q, want %q", results[0].Architecture, "amd64")
		}
		if results[0].Status != "in-progress" {
			t.Errorf("results[0].Status = %q, want %q", results[0].Status, "in-progress")
		}
		if results[0].URL != "https://autopkgtest.ubuntu.com/running" {
			t.Errorf("results[0].URL = %q", results[0].URL)
		}

		if results[1].Architecture != "arm64" {
			t.Errorf("results[1].Architecture = %q, want %q", results[1].Architecture, "arm64")
		}
		if results[1].Status != "pass" {
			t.Errorf("results[1].Status = %q, want %q", results[1].Status, "pass")
		}
		if results[1].URL != "https://objectstorage.example.com/log.gz" {
			t.Errorf("results[1].URL = %q", results[1].URL)
		}
		// Message should be the sanitized (HTML-stripped) version
		if strings.Contains(results[0].Message, "<a") || strings.Contains(results[0].Message, "<span") {
			t.Errorf("Message should not contain HTML: %q", results[0].Message)
		}
		if !strings.Contains(results[0].Message, "autopkgtest for alembic") {
			t.Errorf("Message should contain the plain-text content: %q", results[0].Message)
		}
	})

	t.Run("not a regression status", func(t *testing.T) {
		line := `autopkgtest for glance/2:31.0.0: ` +
			`<a href="https://example.com/armhf">armhf</a>: ` +
			`<a href="https://example.com/log.gz"><span style="background:#e5c545">Not a regression</span></a>`

		results := parseAutopkgtestLine(line)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != "not-regression" {
			t.Errorf("Status = %q, want %q", results[0].Status, "not-regression")
		}
	})

	t.Run("plain text autopkgtest line falls back", func(t *testing.T) {
		line := "autopkgtest for nova/1:29.1.0-0ubuntu1 failed on amd64"
		results := parseAutopkgtestLine(line)
		if len(results) != 1 {
			t.Fatalf("expected 1 fallback result, got %d", len(results))
		}
		if results[0].Status != "unknown" {
			t.Errorf("Status = %q, want %q", results[0].Status, "unknown")
		}
		if results[0].Package != "nova/1:29.1.0-0ubuntu1 failed on amd64" {
			t.Errorf("Package = %q", results[0].Package)
		}
	})

	t.Run("control characters in href are sanitized", func(t *testing.T) {
		line := `autopkgtest for pkg/1.0: ` +
			`<a href="https://example.com/arch">amd64</a>: ` +
			"<a href=\"https://evil.com/\x1b]8;;inject\x07log\"><span>Pass</span></a>"

		results := parseAutopkgtestLine(line)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if strings.ContainsAny(results[0].URL, "\x1b\x07") {
			t.Errorf("URL should not contain control characters: %q", results[0].URL)
		}
	})
}

func TestNormalizeAutopkgtestStatus(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"Pass", "pass"},
		{"Test in progress", "in-progress"},
		{"No test results", "no-results"},
		{"Not a regression", "not-regression"},
		{"Always failed", "always-failed"},
		{"Regression", "regression"},
		{"  Pass  ", "pass"},
		{"PASS", "pass"},
		{"Test in progress (will not be considered a regression)", "in-progress"},
		{"SomethingNew", "somethingnew"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := normalizeAutopkgtestStatus(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeAutopkgtestStatus(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExtractTriggeringPackage(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"autopkgtest for alembic/1.18.4-1: rest", "alembic/1.18.4-1"},
		{"autopkgtest for nova/1:29.1.0-0ubuntu1: rest", "nova/1:29.1.0-0ubuntu1"},
		{"no match here", ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := extractTriggeringPackage(tt.line)
			if got != tt.want {
				t.Errorf("extractTriggeringPackage() = %q, want %q", got, tt.want)
			}
		})
	}
}
