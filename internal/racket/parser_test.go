package racket

import (
	"strings"
	"testing"
)

func TestParseAndFormatProgram(t *testing.T) {
	source := `#lang racket

(define (square x)
  (* x x))

'(a "b")
`

	result := Parse(source)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", result.Diagnostics)
	}

	formatted := Format(result.Root)
	if !strings.Contains(formatted, "#lang racket") {
		t.Fatalf("formatted source dropped #lang: %q", formatted)
	}
	if !strings.Contains(formatted, "(define (square x) (* x x))") {
		t.Fatalf("formatted source dropped define form: %q", formatted)
	}
	if !strings.Contains(formatted, "'(a \"b\")") {
		t.Fatalf("formatted source dropped quote form: %q", formatted)
	}
}

func TestParseRecoversFromMissingDelimiter(t *testing.T) {
	result := Parse("(define (square x)\n  (* x x)\n")
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic")
	}
	if len(result.Root.Children) != 1 {
		t.Fatalf("expected one partial top-level form, got %d", len(result.Root.Children))
	}

	formatted := Format(result.Root)
	if !strings.Contains(formatted, "(define") {
		t.Fatalf("expected partial formatted source, got %q", formatted)
	}
}

func TestDatumCommentIsSkipped(t *testing.T) {
	result := Parse("#;(+ 1 2)\n(displayln 3)")
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", result.Diagnostics)
	}
	formatted := Format(result.Root)
	if strings.Contains(formatted, "+") {
		t.Fatalf("datum comment was not skipped: %q", formatted)
	}
	if !strings.Contains(formatted, "(displayln 3)") {
		t.Fatalf("remaining form was not preserved: %q", formatted)
	}
}
