package lower

import (
	"testing"

	"github.com/vertex-language/objv/analyzer"
)

// The builtin table is written twice — once as signatures in the analyzer and
// once as verbs here — because the two phases want different halves of the
// same fact and neither half implies the other. What they cannot be is
// different lists: a name the analyzer types and this does not emit reaches
// lowering as a call to a symbol no object file has, which fails at the link
// with a name the user never wrote.
func TestBuiltinsAreLowered(t *testing.T) {
	typed := map[string]bool{}
	for _, name := range analyzer.BuiltinNames() {
		typed[name] = true
		if _, ok := builtinOps[name]; !ok {
			t.Errorf("the analyzer types %s; lower does not emit it", name)
		}
	}
	for name := range builtinOps {
		if !typed[name] {
			t.Errorf("lower emits %s; the analyzer does not type it", name)
		}
	}
}
