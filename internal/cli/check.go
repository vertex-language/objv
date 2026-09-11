package cli

import (
	"flag"
	"fmt"
	"io"
)

// cmdCheck runs the front end and prints what it found. No output but
// diagnostics, and an exit code that says whether any were errors.
func cmdCheck(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	names := fs.Args()
	if len(names) == 0 {
		names = []string{"-"}
	}
	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}

	// Every input is checked even after one of them fails, so a check of
	// four files reports four files' mistakes rather than the first one's.
	code := exitOK
	for _, name := range names {
		in, err := input(name)
		if err != nil {
			fmt.Fprintln(stderr, "objv:", err)
			code = maxCode(code, exitUsage)
			continue
		}
		diags, err := c.Check(in)
		if err != nil {
			fmt.Fprintln(stderr, "objv:", err)
			code = maxCode(code, exitUsage)
			continue
		}
		if printDiags(stderr, diags) {
			code = maxCode(code, exitDiags)
		}
	}
	return code
}
