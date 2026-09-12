package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/vertex-language/objv"
)

// cmdRun builds a program into a temporary directory and runs it.
// Arguments after '--' are forwarded to the built binary.
func cmdRun(args []string, stdout, stderr io.Writer) int {
	args, progArgs := splitDashDash(args)

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var b buildFlags
	b.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	names := fs.Args()
	if len(names) == 0 {
		// run cannot read source from stdin because the spawned process inherits stdin.
		fmt.Fprintln(stderr, "objv run: needs a file; the program it builds inherits "+
			"standard input, so the source cannot come from there too")
		return exitUsage
	}

	// Building for another target is fine; running the result here is not.
	// The library will happily produce it — this is the one thing that is
	// this machine's business rather than the compiler's.
	if b.pp.target != objv.HostName() {
		fmt.Fprintf(stderr, "objv run: %s is not this machine; build it and run it there\n",
			b.pp.target)
		return exitUsage
	}

	c, err := b.pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}
	ins, err := inputs(names)
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}

	dir, err := os.MkdirTemp("", "objv-run-")
	if err != nil {
		fmt.Fprintln(stderr, "objv run:", err)
		return exitUsage
	}
	defer os.RemoveAll(dir)

	exe := filepath.Join(dir, "a.out")
	if err := c.Build(b.params(ins, exe)); err != nil {
		return report(err, stderr)
	}

	cmd := exec.Command(exe, progArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// The program's exit code is the answer, not objv's: a program
			// that exits 1 did not fail to build.
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, "objv run:", err)
		return exitUsage
	}
	return exitOK
}

// splitDashDash cuts the argument list at the first bare "--".
func splitDashDash(args []string) (mine, theirs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}
