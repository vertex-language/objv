package analyzer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// The check corpus asks two questions, and a file's name says which.
//
//	ok-*    must produce no errors
//	bad-*   must produce the errors it names, and no others
//
// A bad file names each one where it happens: a line carrying
// `// expect: <text>` must draw a diagnostic on that line whose message
// contains the text. That is clang's -verify in miniature, and it is worth
// the machinery — a test that only counts diagnostics passes when the
// compiler reports the right number of the wrong things.
func TestCheckCorpus(t *testing.T) {
	prelude, err := os.ReadFile("../tests/check/prelude.txt")
	if err != nil {
		t.Fatal(err)
	}
	preludeLines := strings.Count(string(prelude), "\n")

	files, _ := filepath.Glob("../tests/check/*.m")
	if len(files) == 0 {
		t.Fatal("no files in ../tests/check")
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			body, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			src := append(append([]byte{}, prelude...), body...)
			f := token.NewFile(name, src)
			file, diags := parser.ParseFile(f, 0)
			for _, d := range diags {
				t.Errorf("parse: %s", d.Print(f))
			}

			mode := analyzer.Mode(0)
			if strings.Contains(string(body), "This file is checked with ARC on") ||
				strings.Contains(filepath.Base(name), "arc") {
				mode |= analyzer.ARC
			}
			_, ds := analyzer.Check(f, file, types.LP64(), mode)

			// What the file asked for, by line of the file itself. A line
			// may carry several markers: one mistake often draws one
			// diagnostic per thing it broke.
			want := map[int][]string{}
			for i, line := range strings.Split(string(body), "\n") {
				for _, part := range strings.Split(line, "// expect:")[1:] {
					if k := strings.Index(part, "// expect:"); k >= 0 {
						part = part[:k]
					}
					want[i+1] = append(want[i+1], strings.TrimSpace(part))
				}
			}

			got := map[int][]string{}
			for _, d := range ds {
				if d.Severity != token.Error && d.Severity != token.Warn {
					continue
				}
				line := f.Position(d.Pos).Line - preludeLines
				got[line] = append(got[line], d.Message)
			}

			if strings.HasPrefix(filepath.Base(name), "ok-") {
				for _, d := range ds {
					if d.Severity == token.Error {
						t.Errorf("unexpected: %s", d.Print(f))
					}
				}
				return
			}

			for line, texts := range want {
				for _, text := range texts {
					found := false
					for _, msg := range got[line] {
						if strings.Contains(msg, text) {
							found = true
						}
					}
					if !found {
						t.Errorf("line %d: want a diagnostic containing %q, got %v",
							line, text, got[line])
					}
				}
			}
			for line, msgs := range got {
				if _, asked := want[line]; asked {
					continue
				}
				for _, m := range msgs {
					t.Errorf("line %d: unexpected diagnostic %q", line, m)
				}
			}
		})
	}
}
