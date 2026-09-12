package cli

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vertex-language/objv"
	"github.com/vertex-language/objv/preprocessor"
	"github.com/vertex-language/objv/token"
)

// Diagnostic formatting and source snippet rendering for the CLI.

// printDiags renders each diagnostic and reports whether any was an error.
func printDiags(w io.Writer, diags []objv.Diagnostic) bool {
	for _, d := range diags {
		fmt.Fprintln(w, d.String())
		printSiteSnippet(w, d.Site)
		for _, n := range d.Notes {
			fmt.Fprintf(w, "%s: note: %s\n", objv.SiteString(n.Site), n.Msg)
			printSiteSnippet(w, n.Site)
		}
		printIncludeChain(w, d.Site)
	}
	return objv.HasErrors(diags)
}

func printSiteSnippet(w io.Writer, s preprocessor.Site) {
	if s.Origin == nil || s.Origin.File == nil || !s.Pos.IsValid() {
		return
	}
	printSnippetAt(w, s.Origin.File, s.Pos, s.End)
}

// printIncludeChain prints the "In file included from" trail back to the main file.
func printIncludeChain(w io.Writer, s preprocessor.Site) {
	if s.Origin == nil {
		return
	}
	for child := s.Origin; child.Parent != nil; child = child.Parent {
		parent := child.Parent
		if parent.File == nil {
			continue
		}
		p := parent.File.Position(child.IncludePos)
		fmt.Fprintf(w, "    in file included from %s:%d\n", parent.Name(), p.Line)
	}
}

// printSnippetAt underlines one span, in Raw (as-typed) coordinates, so the
// caret lands on what the user wrote even through trigraphs and splices.
func printSnippetAt(w io.Writer, f *token.File, pos, end token.Pos) {
	src := f.Source()
	p := f.Position(pos)

	// The raw line containing the start of the span.
	lo := p.Offset - (p.Column - 1)
	hi := p.Offset
	if lo < 0 || lo > len(src) {
		return
	}
	for hi < len(src) && src[hi] != '\n' && src[hi] != '\r' {
		hi++
	}
	line := src[lo:hi]

	// Underline width: the raw extent, clamped to this line. Raw widens over
	// splices and trigraphs, so ??< underlines all three bytes.
	width := len(f.Raw(pos, end))
	if p.Column-1+width > len(line) {
		width = len(line) - (p.Column - 1)
	}
	if width < 1 {
		width = 1
	}

	// Tabs stay tabs in the pad line so the caret column matches.
	pad := make([]byte, p.Column-1)
	for i := range pad {
		if line[i] == '\t' {
			pad[i] = '\t'
		} else {
			pad[i] = ' '
		}
	}

	fmt.Fprintf(w, "    %s\n    %s%s\n", line, pad, bytes.Repeat([]byte("^"), width))
}
