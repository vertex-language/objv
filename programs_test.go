package objv_test

// The programs corpus: whole Objective-C programs, built twice and run.
//
// Every other corpus asks a question about one layer — does it parse, does it
// typecheck, does it become the right IR. This one asks the only question a
// user asks, which is whether the program does what it does under clang. So
// nothing here writes down an expected value. A number beside a program is a
// claim that has to be maintained by hand and is wrong the moment the program
// drifts; clang's own output is a claim that maintains itself, and a
// disagreement with it is a bug in this compiler by definition.
//
// The files are meant to look like code someone would write — a word counter,
// a settings store, a little event bus — and to be dense in the constructs
// that meet each other only in real programs: a block captured by a
// collection, a category on a framework class, a property whose setter copies,
// a protocol dispatched through `id`, an exception crossing three frames.
// Where a file is doing something for the test's sake rather than the
// program's, it says so.
//
// Every program is built in both memory models unless it says otherwise, with
// `// mode: arc` or `// mode: mrr` on a line of its own. Deterministic output
// only: no clock, no addresses, no hash order, no concurrency.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vertex-language/objv"
)

func TestPrograms(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the binaries these produce run on arm64 macOS")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang is the oracle and is not on PATH")
	}

	files, _ := filepath.Glob("tests/programs/*.m")
	if len(files) == 0 {
		t.Fatal("no files in tests/programs")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, arc := range modesOf(string(src)) {
				name := "mrr"
				if arc {
					name = "arc"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					want := buildWithClang(t, clang, path, dir, arc)
					got := buildWithObjv(t, path, dir, arc)
					if got.out != want.out {
						t.Errorf("output differs from clang's\n--- objv ---\n%s\n--- clang ---\n%s",
							got.out, want.out)
					}
					if got.code != want.code {
						t.Errorf("exit status %d; clang's build exits %d", got.code, want.code)
					}
				})
			}
		})
	}
}

// modesOf reads the `// mode:` line, defaulting to both.
func modesOf(src string) []bool {
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "// mode:")
		if !ok {
			continue
		}
		switch strings.TrimSpace(rest) {
		case "arc":
			return []bool{true}
		case "mrr":
			return []bool{false}
		}
	}
	return []bool{false, true}
}

// frameworksOf reads the `// frameworks:` line. Foundation is always linked;
// a program that needs another says so.
func frameworksOf(src string) []string {
	out := []string{"Foundation"}
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "// frameworks:")
		if !ok {
			continue
		}
		out = append(out, strings.Fields(rest)...)
	}
	return out
}

type programRun struct {
	out  string
	code int
}

func buildWithObjv(t *testing.T, path, dir string, arc bool) programRun {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "objv-prog")
	c := objv.Compiler{ARC: arc}
	err = c.Build(objv.BuildParams{
		Output:     bin,
		Inputs:     []objv.Input{objv.File(path)},
		Frameworks: frameworksOf(string(src)),
	})
	if err != nil {
		t.Fatalf("objv build: %v", err)
	}
	return runProgram(t, bin)
}

func buildWithClang(t *testing.T, clang, path, dir string, arc bool) programRun {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "clang-prog")
	// The selector stubs are clang's calling-convention optimization for a
	// send, not part of the ABI; objv calls objc_msgSend directly, and a
	// link of the two together would need the stubs' symbols. Nothing about
	// the program changes.
	args := []string{"-fno-objc-msgsend-selector-stubs", "-Wno-everything", "-o", bin, path}
	if arc {
		args = append(args, "-fobjc-arc")
	}
	for _, f := range frameworksOf(string(src)) {
		args = append(args, "-framework", f)
	}
	if out, err := exec.Command(clang, args...).CombinedOutput(); err != nil {
		t.Fatalf("clang build: %v\n%s", err, out)
	}
	return runProgram(t, bin)
}

func runProgram(t *testing.T, bin string) programRun {
	t.Helper()
	cmd := exec.Command(bin)
	out, _ := cmd.Output()
	return programRun{out: string(out), code: cmd.ProcessState.ExitCode()}
}
