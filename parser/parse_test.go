package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// TestCorpus parses every file in tests/syntax. The corpus asks one
// question — does it parse? — so a diagnostic from any of them is a failure,
// and nothing in it has to mean anything or run.
func TestCorpus(t *testing.T) {
	files, err := filepath.Glob("../tests/syntax/*.m")
	if err != nil || len(files) == 0 {
		t.Fatal("no test files found in ../tests/syntax/*.m")
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			f := token.NewFile(name, src)
			file, diags := ParseFile(f, 0)
			for _, d := range diags {
				t.Errorf("%s", d.Print(f))
			}
			if file == nil {
				t.Fatal("nil tree")
			}
			if len(file.Decls) == 0 {
				t.Error("no declarations")
			}

			// Every node is reachable, has a real extent, and dumps.
			n := 0
			ast.Inspect(file, func(node ast.Node) bool {
				n++
				if !node.Pos().IsValid() || node.End() <= node.Pos() {
					t.Errorf("%T has an empty span", node)
					return false
				}
				return true
			})
			if n == 0 {
				t.Error("no nodes")
			}
			var b strings.Builder
			if err := ast.Fdump(&b, f, file); err != nil {
				t.Errorf("Fdump: %v", err)
			}
			if os.Getenv("OBJV_DUMP") != "" {
				t.Log("\n" + b.String())
			}
		})
	}
}
