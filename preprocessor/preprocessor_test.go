package preprocessor

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vertex-language/objv/token"
)

// run preprocesses src as the primary file, with cfg's search lists.
func run(t *testing.T, cfg Config, src string) ([]Token, []Diagnostic) {
	t.Helper()
	p := New(cfg)
	return p.Run(token.NewFile("main.m", []byte(src+"\n")))
}

// out is the spelled output: what --emit mi would print, modulo spacing.
func out(toks []Token) string {
	var b strings.Builder
	for i, t := range toks {
		if t.Kind == token.EOF {
			continue
		}
		if i > 0 && t.Spaced() {
			b.WriteByte(' ')
		}
		b.WriteString(t.Text())
	}
	return b.String()
}

func spelled(t *testing.T, cfg Config, src string) string {
	t.Helper()
	toks, diags := run(t, cfg, src)
	if n := errs(diags); n != 0 {
		t.Fatalf("%q: %d errors: %v", src, n, render(diags))
	}
	return out(toks)
}

func errs(ds []Diagnostic) int {
	n := 0
	for _, d := range ds {
		if d.Severity == token.Error {
			n++
		}
	}
	return n
}

func render(ds []Diagnostic) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Severity.String()+": "+d.Msg)
	}
	return out
}

func wantOut(t *testing.T, cfg Config, src, want string) {
	t.Helper()
	if got := spelled(t, cfg, src); got != want {
		t.Errorf("%q\n got: %q\nwant: %q", src, got, want)
	}
}

// ---- macros ----

func TestObjectAndFunctionMacros(t *testing.T) {
	var cfg Config
	wantOut(t, cfg, "#define N 3\nint a[N];", "int a[3];")
	wantOut(t, cfg, "#define ADD(a, b) ((a) + (b))\nx = ADD(1, 2);", "x = ((1) + (2));")
	// '(' must be adjacent to make a function-like macro.
	wantOut(t, cfg, "#define M (x)\ny = M;", "y = (x);")
	// An empty macro leaves nothing behind.
	wantOut(t, cfg, "#define NOTHING\nint NOTHING x;", "int x;")
}

// Prosser's cases, which are why hide sets ride on the tokens.
func TestHideSets(t *testing.T) {
	var cfg Config
	wantOut(t, cfg, "#define foo(x) bar x\nfoo(foo) (2)", "bar foo (2)")
	wantOut(t, cfg, "#define f(x) x\nf(f) (2)", "f (2)")
	// A macro is disabled while its own expansion is rescanned.
	wantOut(t, cfg, "#define A B\n#define B A\nA", "A")
}

func TestStringizeAndPaste(t *testing.T) {
	var cfg Config
	wantOut(t, cfg, `#define STR(x) #x`+"\n"+`STR(a + b)`, `"a + b"`)
	wantOut(t, cfg, `#define STR(x) #x`+"\n"+`STR("q")`, `"\"q\""`)
	wantOut(t, cfg, "#define CAT(a, b) a ## b\nCAT(x, y)", "xy")
	wantOut(t, cfg, "#define CAT(a, b) a ## b\nCAT(+, +)", "++")

	// A paste that is not one token is one diagnostic, with both operands
	// left standing.
	_, diags := run(t, cfg, "#define CAT(a, b) a ## b\nCAT(+, x)")
	if errs(diags) != 1 {
		t.Errorf("bad paste: %v, want one error", render(diags))
	}
}

// @#x is the Objective-C spelling of #x: one token, and a string object.
func TestStringizeToObject(t *testing.T) {
	toks, diags := run(t, Config{}, "#define KEY(x) @#x\nid k = KEY(count);")
	if errs(diags) != 0 {
		t.Fatalf("%v", render(diags))
	}
	var found *Token
	for i := range toks {
		if toks[i].Kind == token.OBJC_STRING_LIT {
			found = &toks[i]
		}
	}
	if found == nil {
		t.Fatalf("no OBJC_STRING_LIT in %q", out(toks))
	}
	if found.Text() != `@"count"` {
		t.Errorf("text = %q, want %q", found.Text(), `@"count"`)
	}
	// With a space it is an @ and an ordinary string, which is what the
	// tokens say and what §6.1 lets the parser join.
	toks, _ = run(t, Config{}, "#define KEY(x) @ #x\nid k = KEY(count);")
	for _, tk := range toks {
		if tk.Kind == token.OBJC_STRING_LIT {
			t.Errorf("`@ #x` should not mint one token: %q", out(toks))
		}
	}
}

func TestVariadicMacros(t *testing.T) {
	var cfg Config
	wantOut(t, cfg, "#define LOG(fmt, ...) f(fmt, __VA_ARGS__)\nLOG(\"x\", 1, 2);",
		`f("x", 1, 2);`)
	// GNU's comma swallow, which every Objective-C logging macro is written
	// with: an empty variadic argument takes the comma with it.
	wantOut(t, cfg, "#define LOG(fmt, ...) f(fmt, ## __VA_ARGS__)\nLOG(\"x\");", `f("x");`)
	wantOut(t, cfg, "#define LOG(fmt, ...) f(fmt, ## __VA_ARGS__)\nLOG(\"x\", 1);", `f("x", 1);`)
}

func TestSpacingSurvives(t *testing.T) {
	// The first token of an expansion inherits the invocation's spacing; a
	// substituted argument's first token inherits the parameter's.
	wantOut(t, Config{}, "#define add(x, y, z) x + y +z;\nsum = add (1,2, 3)",
		"sum = 1 + 2 +3;")

	// And nothing fuses. The tokens stay four, whatever their adjacency
	// flags say; avoiding the paste when printing them is --emit mi's job,
	// and it can only do it because the stream never lost the boundary.
	toks, _ := run(t, Config{}, "#define PLUS +\n#define EMPTY\n+PLUS -EMPTY-")
	want := []token.Kind{token.ADD, token.ADD, token.SUB, token.SUB}
	if len(toks) != len(want) {
		t.Fatalf("tokens = %v, want four operators", out(toks))
	}
	for i, k := range want {
		if toks[i].Kind != k {
			t.Errorf("token %d = %v, want %v", i, toks[i].Kind, k)
		}
	}
}

// ---- conditionals and #if ----

func TestConditionals(t *testing.T) {
	var cfg Config
	wantOut(t, cfg, "#if 1\na\n#else\nb\n#endif", "a")
	wantOut(t, cfg, "#if 0\na\n#elif 1\nb\n#else\nc\n#endif", "b")
	wantOut(t, cfg, "#define X 2\n#if defined X && X > 1\na\n#endif", "a")
	wantOut(t, cfg, "#ifdef NOPE\na\n#endif", "")
	wantOut(t, cfg, "#if 'A' == 65\na\n#endif", "a")
	// An identifier that survives expansion is 0, keywords included.
	wantOut(t, cfg, "#if sizeof\na\n#else\nb\n#endif", "b")

	// The untaken side of a short circuit does not report.
	if _, d := run(t, cfg, "#if 0 && 1/0\n#endif"); errs(d) != 0 {
		t.Errorf("short circuit: %v", render(d))
	}
	// A taken one does, once.
	if _, d := run(t, cfg, "#if 1/0\n#endif"); errs(d) != 1 {
		t.Errorf("division by zero: %v, want one error", render(d))
	}
	// An unterminated conditional points at the directive that opened it.
	if _, d := run(t, cfg, "#if 1\nx"); errs(d) != 1 {
		t.Errorf("unterminated #if: %v", render(d))
	}
}

// A condition inside a skipped group is not evaluated at all (§6.10.1p6),
// which is what makes the guarded-import idiom safe.
func TestSkippedGroupsAreNotEvaluated(t *testing.T) {
	if _, d := run(t, Config{}, "#if 0\n#if 1/0\n#endif\n#endif"); errs(d) != 0 {
		t.Errorf("%v, want none", render(d))
	}
}

// ---- #include, #import, and the open-once cache ----

func mapFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for k, v := range files {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return m
}

func TestIncludeAndImport(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: mapFS(map[string]string{
		"plain.h":    "int plain;\n",
		"guarded.h":  "#ifndef G\n#define G\nint guarded;\n#endif\n",
		"once.h":     "#pragma once\nint once;\n",
		"imported.h": "int imported;\n",
	})}}}

	// An unguarded header read twice is read twice.
	wantOut(t, cfg, "#include <plain.h>\n#include <plain.h>", "int plain; int plain;")
	// A guarded one is not.
	wantOut(t, cfg, "#include <guarded.h>\n#include <guarded.h>", "int guarded;")
	// #pragma once says the same thing outright.
	wantOut(t, cfg, "#include <once.h>\n#include <once.h>", "int once;")
	// And #import says it at the point of use, which is why Objective-C
	// headers carry no guards.
	wantOut(t, cfg, "#import <imported.h>\n#import <imported.h>", "int imported;")
	// An #import binds the file, so a later #include of it is skipped too.
	wantOut(t, cfg, "#import <imported.h>\n#include <imported.h>", "int imported;")
}

func TestQuotedIncludeLooksBesideTheIncluder(t *testing.T) {
	src := mapFS(map[string]string{"Cache.h": "int cached;\n"})
	cfg := Config{Source: Mount{Name: "src", FS: src}}
	wantOut(t, cfg, `#import "Cache.h"`, "int cached;")

	// The angled form does not look there.
	if _, d := run(t, cfg, "#import <Cache.h>"); errs(d) != 1 {
		t.Errorf("angled import of a sibling header: %v, want not found", render(d))
	}
}

func TestFrameworkIncludes(t *testing.T) {
	sdk := mapFS(map[string]string{
		"Foundation.framework/Headers/Foundation.h":      "#import <Foundation/NSString.h>\nint umbrella;\n",
		"Foundation.framework/Headers/NSString.h":        "int nsstring;\n",
		"Foundation.framework/PrivateHeaders/NSSecret.h": "int secret;\n",
	})
	cfg := Config{Frameworks: []Mount{{Name: "SDK/Frameworks", FS: sdk, System: true}}}

	// The umbrella header, and the header it imports from inside itself.
	wantOut(t, cfg, "#import <Foundation/Foundation.h>", "int nsstring; int umbrella;")
	// PrivateHeaders is searched after Headers.
	wantOut(t, cfg, "#import <Foundation/NSSecret.h>", "int secret;")
	// A framework that is not there names itself.
	_, diags := run(t, cfg, "#import <UIKit/UIKit.h>")
	if errs(diags) != 1 {
		t.Fatalf("%v, want one error", render(diags))
	}
	if !strings.Contains(diags[0].Msg, "UIKit/UIKit.h") {
		t.Errorf("message = %q", diags[0].Msg)
	}

	// With no framework list at all, the note says which flag is missing.
	_, diags = run(t, Config{}, "#import <Foundation/Foundation.h>")
	if len(diags) == 0 || len(diags[0].Notes) == 0 {
		t.Fatalf("want a note: %v", render(diags))
	}
	if !strings.Contains(diags[0].Notes[0].Msg, "-F") {
		t.Errorf("note = %q, want it to name -F", diags[0].Notes[0].Msg)
	}
}

func TestIncludeDiagnostics(t *testing.T) {
	// An @ on the header name is one keystroke from every other line.
	_, diags := run(t, Config{}, `#import @"Cache.h"`)
	if errs(diags) != 1 || len(diags[0].Notes) != 1 {
		t.Fatalf("%v", render(diags))
	}
	if !strings.Contains(diags[0].Notes[0].Msg, "drop the @") {
		t.Errorf("note = %q", diags[0].Notes[0].Msg)
	}
	// An absolute path is the build machine leaking into the source.
	if _, d := run(t, Config{}, `#import "/usr/include/stdio.h"`); errs(d) != 1 {
		t.Errorf("absolute path: %v, want one error", render(d))
	}
}

func TestIncludeNext(t *testing.T) {
	first := mapFS(map[string]string{"limits.h": "#include_next <limits.h>\nint wrapper;\n"})
	second := mapFS(map[string]string{"limits.h": "int platform;\n"})
	cfg := Config{Search: []Mount{
		{Name: "first", FS: first},
		{Name: "second", FS: second},
	}}
	wantOut(t, cfg, "#include <limits.h>", "int platform; int wrapper;")
}

func TestDeps(t *testing.T) {
	cfg := Config{
		TrackDeps: true,
		Search:    []Mount{{Name: "inc", FS: mapFS(map[string]string{"a.h": "#import <b.h>\n", "b.h": "int b;\n"})}},
	}
	p := New(cfg)
	p.Run(token.NewFile("main.m", []byte("#import <a.h>\n")))
	got := p.Deps().Files
	want := []string{"main.m", "inc/a.h", "inc/b.h"}
	if len(got) != len(want) {
		t.Fatalf("deps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("deps[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ---- the interrogation operators ----

func TestHasInclude(t *testing.T) {
	cfg := Config{
		Search:     []Mount{{Name: "inc", FS: mapFS(map[string]string{"there.h": "int there;\n"})}},
		Frameworks: []Mount{{Name: "SDK", FS: mapFS(map[string]string{"Foundation.framework/Headers/Foundation.h": "int f;\n"})}},
	}
	wantOut(t, cfg, "#if __has_include(<there.h>)\nyes\n#endif", "yes")
	wantOut(t, cfg, "#if __has_include(<missing.h>)\nyes\n#else\nno\n#endif", "no")
	// It answers the same question the directive asks — frameworks included.
	wantOut(t, cfg, "#if __has_include(<Foundation/Foundation.h>)\nyes\n#endif", "yes")
	// And asking whether the operator exists is how a portable header asks.
	wantOut(t, cfg, "#ifdef __has_include\nyes\n#endif", "yes")
	wantOut(t, cfg, "#if defined(__has_include)\nyes\n#endif", "yes")
	// Asking does not make the header a dependency.
	p := New(Config{TrackDeps: true, Search: cfg.Search})
	p.Run(token.NewFile("main.m", []byte("#if __has_include(<there.h>)\n#endif\n")))
	if len(p.Deps().Files) != 1 {
		t.Errorf("deps = %v, want the primary file alone", p.Deps().Files)
	}
}

func TestHasFeatureAttributeBuiltin(t *testing.T) {
	cfg := Config{
		Feature:   func(n string) bool { return n == "objc_arc" || n == "objc_generics" },
		Attribute: func(n string) bool { return n == "objc_designated_initializer" },
		Builtin:   func(n string) bool { return n == "__builtin_expect" },
	}
	wantOut(t, cfg, "#if __has_feature(objc_arc)\narc\n#else\nmrr\n#endif", "arc")
	wantOut(t, cfg, "#if __has_feature(objc_nonsense)\nyes\n#else\nno\n#endif", "no")
	// __has_extension falls back to __has_feature when the caller does not
	// distinguish them, which is clang's containment rule.
	wantOut(t, cfg, "#if __has_extension(objc_generics)\nyes\n#endif", "yes")
	wantOut(t, cfg, "#if __has_attribute(objc_designated_initializer)\nyes\n#endif", "yes")
	wantOut(t, cfg, "#if __has_builtin(__builtin_expect)\nyes\n#endif", "yes")

	// A zero Config answers no to all of them, and the header takes its
	// fallback — which is the honest answer for a caller that has not said.
	wantOut(t, Config{}, "#if __has_feature(objc_arc)\narc\n#else\nmrr\n#endif", "mrr")

	// The operand is not macro-expanded: it names a feature, not a macro.
	wantOut(t, cfg, "#define objc_arc 0\n#if __has_feature(objc_arc)\narc\n#endif", "arc")
}

// The idiom the Cocoa headers are written in, end to end.
func TestNSEnumIdiom(t *testing.T) {
	cfg := Config{Feature: func(n string) bool { return n == "objc_fixed_enum" }}
	const src = `#if __has_feature(objc_fixed_enum)
#define NS_ENUM(_type, _name) enum _name : _type _name; enum _name : _type
#else
#define NS_ENUM(_type, _name) _type _name; enum
#endif
typedef NS_ENUM(NSInteger, State) { Idle, Busy };`
	wantOut(t, cfg, src, "typedef enum State : NSInteger State; enum State : NSInteger { Idle, Busy };")
}

// The operators answer where they stand, not only in a controlling
// expression: leaving one in ordinary code would hand the parser an
// identifier no header declares.
func TestOperatorsInText(t *testing.T) {
	cfg := Config{
		Feature: func(n string) bool { return n == "objc_arc" },
		Search:  []Mount{{Name: "inc", FS: mapFS(map[string]string{"there.h": "int there;\n"})}},
	}
	wantOut(t, cfg, "int x = __has_feature(objc_arc);", "int x = 1;")
	wantOut(t, cfg, "return __has_feature(objc_arc) ? 0 : 1;", "return 1 ? 0 : 1;")
	wantOut(t, cfg, "int y = __has_include(<there.h>) + __has_include(<gone.h>);", "int y = 1 + 0;")
	// Without a '(' it is an ordinary identifier, which is what
	// `#ifdef __has_include` tests for.
	wantOut(t, cfg, "int z = __has_feature;", "int z = __has_feature;")
}

func TestIsTarget(t *testing.T) {
	cfg := Config{Triple: Triple{Arch: "arm64", Vendor: "apple", OS: "macos", Environment: ""}}
	wantOut(t, cfg, "#if __is_target_arch(arm64)\nyes\n#endif", "yes")
	wantOut(t, cfg, "#if __is_target_arch(aarch64)\nyes\n#endif", "yes") // same thing
	wantOut(t, cfg, "#if __is_target_os(macosx)\nyes\n#endif", "yes")
	wantOut(t, cfg, "#if __is_target_environment(simulator)\nyes\n#else\nno\n#endif", "no")
	wantOut(t, cfg, "#if __has_builtin(__is_target_arch)\nyes\n#endif", "yes")
}

// ---- pragmas ----

func TestPragmaPassesThrough(t *testing.T) {
	toks, diags := run(t, Config{}, "#pragma clang assume_nonnull begin\nint x;")
	if errs(diags) != 0 {
		t.Fatalf("%v", render(diags))
	}
	if got := out(toks); got != "#pragma clang assume_nonnull begin int x;" {
		t.Errorf("out = %q", got)
	}
	if toks[0].Kind != token.HASH || !toks[0].StartsLine() {
		t.Errorf("the '#' must open a logical line, or the line re-enters as garbage")
	}
}

// _Pragma is the spelling a macro must use, and NS_ASSUME_NONNULL_BEGIN is
// the reason phase 4 cannot do without it.
func TestPragmaOperator(t *testing.T) {
	wantOut(t, Config{}, `_Pragma("clang assume_nonnull begin")`,
		"#pragma clang assume_nonnull begin")

	const src = `#define NS_ASSUME_NONNULL_BEGIN _Pragma("clang assume_nonnull begin")
#define NS_ASSUME_NONNULL_END _Pragma("clang assume_nonnull end")
NS_ASSUME_NONNULL_BEGIN
int x;
NS_ASSUME_NONNULL_END`
	wantOut(t, Config{}, src,
		"#pragma clang assume_nonnull begin int x; #pragma clang assume_nonnull end")

	// The operands stay on the pragma's own line. A FlagNLBefore among them
	// prints as `#pragma` alone, which re-enters as a null directive and a
	// stray identifier.
	toks, _ := run(t, Config{}, `_Pragma("clang assume_nonnull begin")`+"\nint x;")
	for i, tk := range toks {
		if i == 0 {
			if !tk.StartsLine() {
				t.Error("the '#' opens a logical line")
			}
			continue
		}
		if tk.Kind == token.INT {
			break // the `int x;` after the pragma legitimately starts a line
		}
		if tk.StartsLine() {
			t.Errorf("token %d (%q) starts a line inside the pragma", i, tk.Text())
		}
	}

	// Escapes are undone as §6.10.9 says.
	wantOut(t, Config{}, `_Pragma("message(\"hi\")")`, `#pragma message("hi")`)

	// _Pragma("once") is #pragma once, about the same file.
	cfg := Config{Search: []Mount{{Name: "inc", FS: mapFS(map[string]string{
		"o.h": "_Pragma(\"once\")\nint o;\n",
	})}}}
	wantOut(t, cfg, "#include <o.h>\n#include <o.h>", "int o;")

	// A non-literal operand is one diagnostic.
	if _, d := run(t, Config{}, "_Pragma(x)"); errs(d) != 1 {
		t.Errorf("_Pragma(x): %v, want one error", render(d))
	}
}

// ---- predefined macros ----

func TestPredefines(t *testing.T) {
	wantOut(t, Config{}, "#if defined(__OBJC__) && __OBJC2__\nobjc\n#endif", "objc")
	wantOut(t, Config{}, "#ifdef __GNUC__\ngnu\n#endif", "gnu")
	wantOut(t, Config{}, "__STDC_VERSION__", "201112L")
	wantOut(t, Config{Std: C17}, "__STDC_VERSION__", "201710L")

	// __FILE__ and __LINE__ are computed, and __FILE__ is never absolute.
	wantOut(t, Config{}, "__FILE__", `"main.m"`)
	wantOut(t, Config{}, "\n\n__LINE__", "3")

	// A -D goes through the same #define grammar a directive does.
	cfg := Config{Predefines: []Predefine{
		{Text: "DEBUG"}, {Text: "LEVEL=3"}, {Text: "MAX(a,b)=((a)>(b)?(a):(b))"},
	}}
	wantOut(t, cfg, "#if DEBUG && LEVEL == 3\nyes\n#endif", "yes")
	wantOut(t, cfg, "x = MAX(1, 2);", "x = ((1)>(2)?(1):(2));")

	// -U removes one, in command-line order.
	cfg = Config{Predefines: []Predefine{{Text: "A=1"}, {Kind: PredefineUndef, Text: "A"}}}
	wantOut(t, cfg, "#ifdef A\nyes\n#else\nno\n#endif", "no")
}

// IBAction must reach the parser as the keyword `void`, not as an identifier
// spelled "void" — which is what minting a body by hand would produce.
func TestInterfaceBuilderMacros(t *testing.T) {
	toks, diags := run(t, Config{}, "- (IBAction)tap:(id)sender;")
	if errs(diags) != 0 {
		t.Fatalf("%v", render(diags))
	}
	found := false
	for _, tk := range toks {
		if tk.Kind == token.VOID {
			found = true
		}
	}
	if !found {
		t.Errorf("IBAction expanded to %q, with no VOID keyword", out(toks))
	}
	wantOut(t, Config{}, "@property (weak) IBOutlet UILabel *label;",
		"@property (weak) UILabel *label;")
}

// ---- what phase 4 does not touch ----

func TestObjectiveCTokensPassThrough(t *testing.T) {
	const src = `@interface Greeter : NSObject
- (void)greet:(NSString *)name;
@end
@import Foundation;`
	toks, diags := run(t, Config{}, src)
	if errs(diags) != 0 {
		t.Fatalf("%v", render(diags))
	}
	kinds := map[token.Kind]int{}
	for _, tk := range toks {
		kinds[tk.Kind]++
	}
	// @import is a module import (§4.4), not a preprocessing directive: it
	// carries no '#' and survives as the token it was scanned as.
	for _, k := range []token.Kind{token.AT_INTERFACE, token.AT_END, token.AT_IMPORT} {
		if kinds[k] != 1 {
			t.Errorf("%v: %d, want 1 — phase 4 must not touch it", k, kinds[k])
		}
	}
}

func TestLiteralsLeaveUndecoded(t *testing.T) {
	toks, _ := run(t, Config{}, `id s = @"hi"; int n = 0x1Fu;`)
	for _, tk := range toks {
		switch tk.Kind {
		case token.OBJC_STRING_LIT:
			if tk.Text() != `@"hi"` {
				t.Errorf("string = %q", tk.Text())
			}
		case token.INT_LIT:
			if tk.Text() != "0x1Fu" {
				t.Errorf("number = %q", tk.Text())
			}
		}
	}
}

// A malformed literal inside a skipped group is nobody's mistake: the
// scanner defers it and phase 4 never converts it.
func TestSkippedGroupsKeepTheirSecrets(t *testing.T) {
	if _, d := run(t, Config{}, "#if 0\nint a[0779];\nid x = @NSFoo;\n#endif"); len(d) != 0 {
		t.Errorf("%v, want none", render(d))
	}
}

// ---- positions ----

func TestPositionsSurviveExpansion(t *testing.T) {
	// A token from the replacement list, not from an argument: an argument's
	// tokens are the user's own and keep their own spans, while a body
	// token exists only because the macro was invoked.
	const src = "#define BOOM() explode\nBOOM()"
	toks, _ := run(t, Config{}, src)
	var last Token
	for _, tk := range toks {
		if tk.Kind == token.IDENT && tk.Text() == "explode" {
			last = tk
		}
	}
	if last.Kind != token.IDENT {
		t.Fatalf("no expanded token in %q", out(toks))
	}
	// A diagnostic points at the invocation the user typed.
	site := last.Site()
	if site.Origin == nil || site.Origin.Name() != "main.m" {
		t.Fatalf("site = %+v", site)
	}
	if line := site.Origin.File.Position(site.Pos).Line; line != 2 {
		t.Errorf("site line = %d, want 2 (the invocation)", line)
	}
	// And the notes walk out through the definition.
	if len(last.Notes()) == 0 {
		t.Error("want a note siting the replacement list")
	}
}

func TestDiagnosticsCarryTheIncludeChain(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: mapFS(map[string]string{
		"bad.h": "#error broken\n",
	})}}}
	_, diags := run(t, cfg, "#import <bad.h>")
	if errs(diags) != 1 {
		t.Fatalf("%v", render(diags))
	}
	org := diags[0].Site.Origin
	if org == nil || org.Name() != "inc/bad.h" {
		t.Fatalf("origin = %v", org.Name())
	}
	if org.Parent == nil || org.Parent.Name() != "main.m" {
		t.Errorf("want the chain back to main.m, got %v", org.Parent.Name())
	}
	if org.Depth() != 1 {
		t.Errorf("depth = %d, want 1", org.Depth())
	}
}

// A warning in a system header is the header's mistake, not each import of
// it: once per header, however many files reach it.
func TestSystemHeaderWarnsOnce(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "sdk", System: true, FS: mapFS(map[string]string{
		"noisy.h": "#warning old compiler\n",
		"a.h":     "#include <noisy.h>\n",
		"b.h":     "#include <noisy.h>\n",
	})}}}
	_, diags := run(t, cfg, "#include <a.h>\n#include <b.h>")
	n := 0
	for _, d := range diags {
		if d.Severity == token.Warn {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d warnings %v, want one", n, render(diags))
	}
}

func TestDeterminism(t *testing.T) {
	cfg := Config{Search: []Mount{{Name: "inc", FS: mapFS(map[string]string{
		"a.h": "#import <b.h>\nint a;\n", "b.h": "int b;\n",
	})}}}
	const src = "#import <a.h>\n__DATE__ __TIME__ __COUNTER__ __COUNTER__"
	first := spelled(t, cfg, src)
	for i := 0; i < 3; i++ {
		if got := spelled(t, cfg, src); got != first {
			t.Fatalf("run %d differs:\n %q\n %q", i, got, first)
		}
	}
	if !strings.Contains(first, "0 1") {
		t.Errorf("__COUNTER__ should count: %q", first)
	}
}
