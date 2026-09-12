// Package cli implements the objv command-line interface.
//
// Run is the main entry point, wrapping the core objv compiler pipeline with
// CLI flag parsing, I/O handling, and diagnostic formatting.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/vertex-language/objv"
)

// Exit codes, gcc-shaped so `objv check f.m && echo ok` means what it looks
// like.
const (
	exitOK    = 0 // no error diagnostics
	exitDiags = 1 // the input had errors
	exitUsage = 2 // the invocation or I/O had errors
)

const usage = `objv — the Vertex Objective-C compiler

Usage:

    objv build   [flags] [files...]         compile and link; with --emit, stop earlier
    objv run     [flags] file [-- args...]  build to a temporary path and run it
    objv check   [flags] [files...]         preprocess, parse, analyze; print diagnostics
    objv ast     [flags] [file]             parse and dump the syntax tree
    objv tokens  [flags] [file]             dump the token stream
    objv env     [flags]                    print the resolved search lists and the SDK
    objv version                            print the compiler's version

A file of "-" (or no file) reads standard input, except for run: the
program it builds inherits standard input, so the source cannot use it.

A .m or .c file runs through objv's own preprocessor; a .mi or .i file
(or stdin) enters the pipeline above it. -pp and -no-pp override the
extension.

Common flags:
    -target T       target to compile for (default: this host)
    -I dir          add an include search directory (repeatable, in order)
    -F dir          add a framework search directory (repeatable, in order)
    -D name[=val]   define a macro (repeatable)
    -U name         undefine a macro (repeatable)
    -include file   process a file before the main input (repeatable)
    -fobjc-arc      compile with automatic reference counting
    -isysroot path  the SDK to compile and link against
    -mmacosx-version-min V   the oldest macOS this build runs on
    -freestanding   builtin headers only; no SDK, no frameworks
    -pp / -no-pp    force preprocessing on or off

Flags for build and run:
    --emit exe      compile and link (the default)
    --emit obj      compile to an object file, one per input
    --emit vir      stop after phase 7 and print the lowered VIR module
    --emit mi       stop after phase 4 and print preprocessed source
    -o file         write output here ("-" is standard output, for mi and vir)
    -L dir          add a library search directory (repeatable, in order)
    -l name         link against a library (repeatable, in order)
    -framework name link against a framework (repeatable, in order)
    -entry sym      the program's entry symbol (default: the platform's)
    -static         link a static image

Flags for ast:
    -skip-bodies    skip method and function bodies (fast structural pass)
    -comments       retain comments on the tree

Flags for env:
    -defines        also print the predefined macros

The linker is objv's own, so -target builds and links an executable for
any target objv models, given that platform's headers and libraries.
Running one is another matter: objv run needs this machine's target.

Exit codes: 0 no errors, 1 diagnostics with errors, 2 usage or I/O.
`

// Run executes one objv invocation and returns its exit code. It is the
// package's entire surface: cmd/objv calls it and nothing else.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "build":
		return cmdBuild(rest, stdout, stderr)
	case "run":
		return cmdRun(rest, stdout, stderr)
	case "check":
		return cmdCheck(rest, stderr)
	case "ast":
		return cmdAST(rest, stdout, stderr)
	case "tokens":
		return cmdTokens(rest, stdout, stderr)
	case "env":
		return cmdEnv(rest, stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "objv %s\n", objv.Version)
		return exitOK
	case "help", "-h", "--help", "-help":
		fmt.Fprint(stdout, usage)
		return exitOK
	}
	fmt.Fprintf(stderr, "objv: unknown command %q\n\n%s", verb, usage)
	return exitUsage
}

// input names one thing to compile. "" and "-" mean standard input, which the
// library has no way to read for itself — a pipe is the command line's, and
// what reaches the library is the bytes it carried.
func input(name string) (objv.Input, error) {
	if name == "" || name == "-" {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return objv.Input{}, fmt.Errorf("<stdin>: %w", err)
		}
		return objv.Text("<stdin>", src), nil
	}
	return objv.File(name), nil
}

// inputs is input over a command line, in order.
func inputs(names []string) ([]objv.Input, error) {
	out := make([]objv.Input, 0, len(names))
	for _, name := range names {
		in, err := input(name)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, nil
}

// writeOut sends one artifact to a file or to standard output. "-" and ""
// mean standard output.
func writeOut(name string, stdout io.Writer, data []byte) error {
	if isStdout(name) {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(name, data, 0o666)
}

func isStdout(name string) bool { return name == "" || name == "-" }

func maxCode(a, b int) int {
	if a > b {
		return a
	}
	return b
}
