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
	"strconv"
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

	entries, err := os.ReadDir("tests/programs")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no programs in tests/programs")
	}
	for _, e := range entries {
		p := programOf(t, filepath.Join("tests/programs", e.Name()))
		if len(p.files) == 0 {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			// Not parallel. Each program is four builds against the SDK --
			// two by objv and two by clang -- and running the corpus at
			// once is a few dozen concurrent links, which is a load the
			// machine notices and a speedup nobody needed.
			for _, arc := range p.modes {
				name := "mrr"
				if arc {
					name = "arc"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					want := buildWithClang(t, clang, p, dir, arc)
					got := buildWithObjv(t, clang, p, dir, arc)
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

// A program is one corpus entry: a single file, or a directory of
// translation units that are linked together.
type program struct {
	files      []string // every source, in link order
	foreign    []string // the ones clang compiles in both builds
	frameworks []string
	libraries  []string
	modes      []bool
}

// programOf reads one entry. A directory is a multi-file program, and its
// files are taken in name order so the link order is the one a reader sees.
//
// A file named *.clang.m is compiled by clang in *both* builds, which is what
// makes a directory an interop test rather than only a linking one: half the
// objects come from the compiler this one has to agree with, in one process,
// and the metadata the runtime walks has to be the metadata clang emitted.
func programOf(t *testing.T, path string) program {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var p program
	if !info.IsDir() {
		if filepath.Ext(path) != ".m" {
			return p
		}
		p.files = []string{path}
	} else {
		ents, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if filepath.Ext(e.Name()) == ".m" {
				p.files = append(p.files, filepath.Join(path, e.Name()))
			}
		}
	}

	seen := map[string]bool{"Foundation": true}
	p.frameworks = []string{"Foundation"}
	for _, f := range p.files {
		if strings.HasSuffix(f, ".clang.m") {
			p.foreign = append(p.foreign, f)
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, fw := range markerOf(string(src), "// frameworks:") {
			if !seen[fw] {
				seen[fw] = true
				p.frameworks = append(p.frameworks, fw)
			}
		}
		for _, lib := range markerOf(string(src), "// libraries:") {
			if !seen["-l"+lib] {
				seen["-l"+lib] = true
				p.libraries = append(p.libraries, lib)
			}
		}
		if m := modeOf(string(src)); m != nil {
			p.modes = m
		}
	}
	if p.modes == nil {
		p.modes = []bool{false, true}
	}
	return p
}

func (p program) isForeign(file string) bool {
	for _, f := range p.foreign {
		if f == file {
			return true
		}
	}
	return false
}

// modeOf reads the `// mode:` line, or nil where the file does not say.
func modeOf(src string) []bool {
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
	return nil
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
	out  string
	code int
}

// buildWithObjv compiles every source objv owns, hands the rest to clang, and
// links the lot with objv's own linker.
func buildWithObjv(t *testing.T, clang string, p program, dir string, arc bool) programRun {
	t.Helper()
	bin := filepath.Join(dir, "objv-prog")
	inputs := make([]objv.Input, 0, len(p.files))
	for i, f := range p.files {
		if !p.isForeign(f) {
			inputs = append(inputs, objv.File(f))
			continue
		}
		obj := filepath.Join(dir, "foreign"+itoa(i)+".o")
		compileWithClang(t, clang, f, obj, arc)
		inputs = append(inputs, objv.File(obj))
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

func compileWithClang(t *testing.T, clang, src, obj string, arc bool) {
	t.Helper()
	args := append(clangArgs(arc), "-c", "-o", obj, src)
	if out, err := exec.Command(clang, args...).CombinedOutput(); err != nil {
		t.Fatalf("clang -c %s: %v\n%s", src, err, out)
	}
}

// clangArgs is what every clang invocation here carries.
//
// The selector stubs are clang's calling-convention optimization for a send,
// not part of the ABI; objv calls objc_msgSend directly, and a link of the
// two together would need the stubs' symbols. Nothing about the program
// changes.
func clangArgs(arc bool) []string {
	args := []string{"-fno-objc-msgsend-selector-stubs", "-Wno-everything"}
	if arc {
		args = append(args, "-fobjc-arc")
	}
	return args
}

func itoa(n int) string { return strconv.Itoa(n) }

func runProgram(t *testing.T, bin string) programRun {
	t.Helper()
	cmd := exec.Command(bin)
	out, _ := cmd.Output()
	return programRun{out: string(out), code: cmd.ProcessState.ExitCode()}
}
