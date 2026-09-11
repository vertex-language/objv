package lower_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/ir/text"
	"github.com/vertex-language/ir/verify"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/lower"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// The lower corpus asks: does it become the right IR?
//
// A file states what it expects as substrings of the module text:
//
//	// vir: i32.mul
//	// vir-not: memcpy
//
// Substrings and not line numbers, because a lowering has no line to be on:
// one statement becomes several blocks and one expression becomes several
// instructions, and a test anchored to a line would fail on every change to
// the order things are emitted in rather than on a change to what is
// emitted. What the marker names is a fact about the module — this symbol
// exists, this section is used, this instruction is reached — which is what
// a reader of the file wanted to know anyway.
//
// Every file must also lower without a diagnostic and pass verify.Module.
// That is the half of the contract no marker states, and it is the half
// that catches the bugs: an initializer whose shape does not match its
// declared type prints perfectly well.
func TestLowerCorpus(t *testing.T) {
	prelude, err := os.ReadFile("../tests/eval/prelude.txt")
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob("../tests/eval/*.m")
	if len(files) == 0 {
		t.Fatal("no files in ../tests/eval")
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			body, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			src := append(append([]byte{}, prelude...), body...)
			out, mod := lowerSource(t, name, src)

			for _, want := range markers(string(body), "// vir: ") {
				if !strings.Contains(out, want) {
					t.Errorf("module does not contain %q", want)
				}
			}
			for _, bad := range markers(string(body), "// vir-not: ") {
				if strings.Contains(out, bad) {
					t.Errorf("module contains %q, which the file forbids", bad)
				}
			}
			if err := verify.Module(mod); err != nil {
				t.Errorf("verify: %v", err)
			}
		})
	}
}

// TestSyntaxCorpusLowers runs the parser's corpus through lowering as a
// crash test. Most of it does not typecheck without an SDK, so nothing is
// asserted about the output — only that the compiler reaches the end.
func TestSyntaxCorpusLowers(t *testing.T) {
	files, _ := filepath.Glob("../tests/syntax/*.m")
	if len(files) == 0 {
		t.Skip("no syntax corpus")
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			f := token.NewFile(name, src)
			file, _ := parser.ParseFile(f, 0)
			info, _ := analyzer.Check(f, file, types.LP64(), 0)
			mod, _ := lower.Lower(f, file, info, options(filepath.Base(name)))
			if mod == nil {
				t.Fatal("Lower returned no module")
			}
			// The module is partial by construction, so it is formatted
			// rather than verified: what is being tested is that lowering
			// a tree full of unresolved names does not panic.
			_, _ = text.Format(mod)
		})
	}
}

// lowerSource runs the whole front end and returns the module's text.
func lowerSource(t *testing.T, name string, src []byte) (string, *ir.Module) {
	t.Helper()
	f := token.NewFile(name, src)
	file, diags := parser.ParseFile(f, 0)
	for _, d := range diags {
		t.Errorf("parse: %s", d.Print(f))
	}
	// A file says `// arc:` on a line of its own to be read with automatic
	// reference counting. The two memory models are two lowerings of one
	// language, and the retains are the half nothing else here covers.
	mode := analyzer.Mode(0)
	arc := strings.Contains(string(src), "\n// arc\n")
	if arc {
		mode = analyzer.ARC
	}
	info, ds := analyzer.Check(f, file, types.LP64(), mode)
	for _, d := range ds {
		if d.Severity == token.Error {
			t.Errorf("check: %s", d.Print(f))
		}
	}
	opts := options(filepath.Base(name))
	opts.ARC = arc
	mod, lds := lower.Lower(f, file, info, opts)
	for _, d := range lds {
		if d.Severity == token.Error {
			t.Errorf("lower: %s", d.Print(f))
		}
	}
	out, err := text.Format(mod)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	return string(out), mod
}

func options(name string) lower.Options {
	return lower.Options{
		Name:         "m" + strings.Map(identChar, strings.TrimSuffix(name, ".m")),
		Target:       ir.AArch64MacOS,
		Model:        types.LP64(),
		ABI:          runtime.Darwin64(),
		Arch:         runtime.ARM64,
		SymbolPrefix: "_",

		// A deployment target, so that §6.10's @available has something to
		// fold against. 12.0 is old enough that a corpus file can ask for
		// both answers.
		Platform:   runtime.PlatformMacOS,
		Deployment: runtime.OSVersion{Major: 12},
	}
}

func identChar(r rune) rune {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
		return r
	}
	return '_'
}

// markers collects the text after each occurrence of a marker prefix.
func markers(src, prefix string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		i := strings.Index(line, prefix)
		if i < 0 {
			continue
		}
		if s := strings.TrimSpace(line[i+len(prefix):]); s != "" {
			out = append(out, s)
		}
	}
	return out
}
