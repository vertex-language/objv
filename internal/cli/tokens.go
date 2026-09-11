package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/vertex-language/objv"
	"github.com/vertex-language/objv/scanner"
	"github.com/vertex-language/objv/token"
)

func cmdTokens(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tokens", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "objv tokens: one file at a time")
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}
	in, err := input(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}

	// On .m input the stream shown is the re-scan of phase 4's output — what
	// the parser will actually see. -no-pp shows the raw scan of the file as
	// written.
	unit, diags, err := c.Source(in)
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}
	hadErrors := printDiags(stderr, diags)

	toks, scanDiags := scanner.Scan(unit, scanner.ScanComments)
	hadErrors = printDiags(stderr, objv.Sited(unit, scanDiags)) || hadErrors

	for _, t := range toks {
		p := unit.Position(t.Pos)
		fmt.Fprintf(stdout, "%3d:%-3d %-20s", p.Line, p.Column, t.Kind)
		// The spelling, for every token whose kind does not carry it. A
		// directive does: AT_INTERFACE is spelled "@interface" and nothing
		// else, which is why §2.5's closed set is a kind and not an
		// identifier after an '@'.
		switch t.Kind {
		case token.IDENT, token.INT_LIT, token.FLOAT_LIT, token.CHAR_LIT,
			token.STRING_LIT, token.OBJC_STRING_LIT, token.BOOL_LIT,
			token.COMMENT, token.ILLEGAL:
			fmt.Fprintf(stdout, " %s", unit.Slice(t.Pos, t.End))
		}
		fmt.Fprintf(stdout, "%s\n", flagString(t.Flags))
	}
	if hadErrors {
		return exitDiags
	}
	return exitOK
}

func flagString(f token.Flags) string {
	s := ""
	if f.Has(token.FlagAdjacent) {
		s += " [adj]"
	}
	if f.Has(token.FlagNLBefore) {
		s += " [nl]"
	}
	if f.Has(token.FlagDigraph) {
		s += " [digraph]"
	}
	if f.Has(token.FlagSpacedAt) {
		s += " [spaced-at]"
	}
	return s
}
