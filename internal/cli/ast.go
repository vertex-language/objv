package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/parser"
)

func cmdAST(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ast", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	skipBodies := fs.Bool("skip-bodies", false, "skip method and function bodies")
	comments := fs.Bool("comments", false, "retain comments on the tree")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "objv ast: one file at a time")
		return exitUsage
	}

	// -comments must survive phase 4 too: the preprocessor drops COMMENT
	// tokens unless told to keep them.
	pp.keepComments = *comments

	mode := parser.DefaultMode
	if *skipBodies {
		mode |= parser.SkipBodies
	}
	if *comments {
		mode |= parser.ParseComments
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
	file, diags, err := c.Parse(in, mode)
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}
	defer file.Release()

	// The tree is printed even when the parse had errors: a partial tree is
	// what a broken file should dump, and it is usually the thing the reader
	// wanted to see.
	hadErrors := printDiags(stderr, diags)
	if err := ast.Fdump(stdout, file.Unit, file); err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}
	if hadErrors {
		return exitDiags
	}
	return exitOK
}
