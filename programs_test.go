package objv_test

// The corpus: tests/NNN-*.m, one small thing per file, numbered in the order
// they climb: plain C, then Objective-C, then a second pass over each (see
// tests/README.md).
//
// Every file is built twice -- once by objv, once by clang -- run, and the
// stdout and exit status compared. Nothing here writes down an expected
// value: clang's answer is the oracle, and a disagreement with it is a bug in
// this compiler by definition.
//
// A file is built without ARC unless it says otherwise, on a line of its own:
//
//	// mode: arc     // or mrr, or both
//	// frameworks: CoreGraphics
//	// libraries: m
//
// Deterministic output only: no clock, no addresses, no hash order, no
// threads racing to print, no UI.

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vertex-language/objv"
)

func TestCorpus(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the binaries these produce run on arm64 macOS")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang is the oracle and is not on PATH")
	}

	files, err := filepath.Glob("tests/*.m")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no programs in tests/")
	}
	for _, f := range files {
		p := programOf(t, f)
		t.Run(strings.TrimSuffix(filepath.Base(f), ".m"), func(t *testing.T) {
			for _, arc := range p.modes {
				name := "mrr"
				if arc {
					name = "arc"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					want := buildWithClang(t, clang, p, dir, arc)
					got := buildWithObjv(t, p, dir, arc)
					if got.out != want.out {
						t.Errorf("output differs from clang's\n--- objv ---\n%s\n--- clang ---\n%s",
							clip(got.out), clip(want.out))
					}
					if got.status != want.status {
						t.Errorf("objv's build %s; clang's %s", got.status, want.status)
					}
				})
			}
		})
	}
}

// A program is one corpus file.
type program struct {
	files      []string
	frameworks []string
	libraries  []string
	modes      []bool
}

// programOf reads one file's markers. Foundation is always linked.
func programOf(t *testing.T, path string) program {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := program{files: []string{path}, frameworks: []string{"Foundation"}}
	for _, fw := range markerOf(string(src), "// frameworks:") {
		if fw != "Foundation" {
			p.frameworks = append(p.frameworks, fw)
		}
	}
	p.libraries = markerOf(string(src), "// libraries:")
	p.modes = modeOf(string(src))
	return p
}

// modeOf reads the `// mode:` line. Without one a file is built once,
// without ARC: plain C does not care, and the Objective-C files say so.
func modeOf(src string) []bool {
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "// mode:")
		if !ok {
			continue
		}
		switch strings.TrimSpace(rest) {
		case "arc":
			return []bool{true}
		case "both":
			return []bool{false, true}
		}
	}
	return []bool{false}
}

// markerOf reads the words after a `// name:` line. Foundation is always
// linked; a program that needs another framework, or a library, says so.
func markerOf(src, name string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), name)
		if !ok {
			continue
		}
		out = append(out, strings.Fields(rest)...)
	}
	return out
}

type programRun struct {
	out    string
	status string // "exit N" or the signal that ended it
}

func buildWithObjv(t *testing.T, p program, dir string, arc bool) programRun {
	t.Helper()
	bin := filepath.Join(dir, "objv-prog")
	var inputs []objv.Input
	for _, f := range p.files {
		inputs = append(inputs, objv.File(f))
	}
	c := objv.Compiler{ARC: arc}
	if err := c.Build(objv.BuildParams{
		Output: bin, Inputs: inputs,
		Frameworks: p.frameworks, Libraries: p.libraries,
	}); err != nil {
		t.Fatalf("objv build: %v", err)
	}
	return runProgram(t, bin)
}

func buildWithClang(t *testing.T, clang string, p program, dir string, arc bool) programRun {
	t.Helper()
	bin := filepath.Join(dir, "clang-prog")
	args := append(clangArgs(arc), "-o", bin)
	args = append(args, p.files...)
	for _, f := range p.frameworks {
		args = append(args, "-framework", f)
	}
	for _, l := range p.libraries {
		args = append(args, "-l"+l)
	}
	if out, err := exec.Command(clang, args...).CombinedOutput(); err != nil {
		t.Fatalf("clang build: %v\n%s", err, out)
	}
	return runProgram(t, bin)
}

// clangArgs is what every clang invocation here carries.
//
// The selector stubs are clang's calling-convention optimization for a send,
// not part of the ABI; objv calls objc_msgSend directly. -ffp-contract=off
// because objv does not fuse a*b+c, and a last-bit difference there is a
// known gap rather than something every float test should trip over. An
// implicit declaration is an error, so a missing #include cannot hide behind
// clang guessing a signature.
func clangArgs(arc bool) []string {
	args := []string{"-fno-objc-msgsend-selector-stubs", "-ffp-contract=off", "-Wno-everything",
		"-Werror=implicit-function-declaration"}
	if arc {
		args = append(args, "-fobjc-arc")
	}
	return args
}

// runProgram runs a built program with a time limit and a cap on its
// output: a miscompiled loop must fail its own test, not take the whole run
// down with it.
func runProgram(t *testing.T, bin string) programRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	out := &cappedBuffer{limit: 1 << 20, cancel: cancel}
	cmd.Stdout = out
	_ = cmd.Run()
	switch {
	case out.overflow:
		return programRun{out: out.String(), status: "wrote more than 1 MB"}
	case ctx.Err() != nil:
		return programRun{out: out.String(), status: "timed out"}
	}
	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ws.Signaled() {
		return programRun{out: out.String(), status: "killed by " + ws.Signal().String()}
	}
	return programRun{out: out.String(), status: "exit " + strconv.Itoa(ws.ExitStatus())}
}

// cappedBuffer keeps the first limit bytes and stops the program after.
type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
	cancel   func()
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.overflow = true
		b.cancel()
		return 0, io.ErrShortWrite
	}
	return b.Buffer.Write(p)
}

// clip keeps a failure message readable when a program ran away.
func clip(s string) string {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) <= 40 {
		return s
	}
	return strings.Join(lines[:40], "") + "... (" + strconv.Itoa(len(lines)-40) + " more lines)\n"
}
