package lower_test

import (
	"strings"
	"testing"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/ir/text"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/lower"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// The unit tests, for what the corpus cannot ask: the options that change
// the output, the diagnostics, and the shapes that have no home in a file
// of realistic Objective-C.

// build lowers a snippet and returns the module text along with the
// diagnostics, so a test may assert on either.
func build(t *testing.T, src string, opts ...func(*lower.Options)) (string, []token.Diagnostic) {
	t.Helper()
	f := token.NewFile("t.m", []byte(src))
	file, diags := parser.ParseFile(f, 0)
	for _, d := range diags {
		t.Fatalf("parse: %s", d.Print(f))
	}
	info, _ := analyzer.Check(f, file, types.LP64(), 0)
	opt := lower.Options{
		Name: "t", Target: ir.AArch64MacOS, Model: types.LP64(),
		ABI: runtime.Darwin64(), Arch: runtime.ARM64, SymbolPrefix: "_",
	}
	for _, o := range opts {
		o(&opt)
	}
	mod, lds := lower.Lower(f, file, info, opt)
	out, err := text.Format(mod)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	return string(out), lds
}

func mustContain(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("module does not contain %q\n%s", w, out)
		}
	}
}

func TestModuleHeader(t *testing.T) {
	out, _ := build(t, "int x;")
	mustContain(t, out, "module t", `use "aarch64/macos"`, "ptrbits    64")
}

// The module is never nil, even for input that lowered to nothing: a
// partial module is what --emit vir on broken input should print.
func TestEmptyUnitStillProducesAModule(t *testing.T) {
	out, diags := build(t, "")
	if len(diags) != 0 {
		t.Fatalf("diagnostics on an empty unit: %v", diags)
	}
	mustContain(t, out, "module t")
}

// SymbolPrefix is stated rather than derived: nothing below lower renames a
// symbol, and the platform that wants the underscore wants it on every C
// identifier, static ones included.
func TestSymbolPrefix(t *testing.T) {
	// f calls g so that g is emitted at all: a static function nothing
	// mentions is not in the module to have a symbol.
	src := "static int g(void) { return 1; }\nint f(void) { return g(); }\n"
	with, _ := build(t, src)
	mustContain(t, with, "@_f", "@_g")

	without, _ := build(t, src, func(o *lower.Options) { o.SymbolPrefix = "" })
	mustContain(t, without, "func @f", "func @g")
	if strings.Contains(without, "@_f") {
		t.Error("a module built with no prefix has an underscored symbol")
	}
}

// A method's symbol is mangled, because `-[Counter count]` is not an
// identifier and a VIR symbol is.
func TestMethodSymbolIsMangled(t *testing.T) {
	out, diags := build(t, `
__attribute__((objc_root_class)) @interface K @end
@implementation K
- (int)count { return 0; }
+ (int)total:(int)n { return n; }
@end
`)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("lower: %s", d.Message)
		}
	}
	mustContain(t, out, "@__i_K__count", "@__c_K__total_")
}

// The ARC option changes the image-info flags the runtime reads, which is
// how a process knows whether it may run a non-ARC image beside this one.
func TestARCChangesImageInfo(t *testing.T) {
	src := "__attribute__((objc_root_class)) @interface K @end\n@implementation K\n@end\n"
	plain, _ := build(t, src)
	arc, _ := build(t, src, func(o *lower.Options) { o.ARC = true })
	if plain == arc {
		t.Error("ARC did not change the module")
	}
	mustContain(t, plain, "@_L_OBJC_IMAGE_INFO")
	mustContain(t, arc, "@_L_OBJC_IMAGE_INFO")
}

// One objc_msgSend has as many signatures as there are methods: the
// trampoline is imported once and named per call-site shape.
func TestSendSignaturesAreNamedPerShape(t *testing.T) {
	out, _ := build(t, `
__attribute__((objc_root_class)) @interface K
- (int)a;
- (void)b:(int)n;
- (int)a2;
@end
int f(K *k) { [k b:1]; return [k a] + [k a2]; }
`)
	if n := strings.Count(out, "import func @_objc_msgSend("); n != 1 {
		t.Errorf("objc_msgSend imported %d times, want 1", n)
	}
	mustContain(t, out,
		"type @msgsig_ptr_ptr_ri32 func (ptr, ptr) i32",
		"type @msgsig_ptr_ptr_i32 func (ptr, ptr, i32)")
	// Two methods of one shape share one type declaration.
	if n := strings.Count(out, "type @msgsig_ptr_ptr_ri32"); n != 1 {
		t.Errorf("the same signature was declared %d times", n)
	}
}

// A selector, a class name and a string are interned once per unit: that is
// what makes a send two instructions rather than a lookup.
func TestPoolsIntern(t *testing.T) {
	out, _ := build(t, `
__attribute__((objc_root_class)) @interface K
- (int)a;
@end
int f(K *k) { return [k a] + [k a]; }
const char *g(void) { return "same"; }
const char *h(void) { return "same"; }
`)
	if n := strings.Count(out, `= "a"`); n != 1 {
		t.Errorf("the selector name was emitted %d times, want 1", n)
	}
	if n := strings.Count(out, `= "same"`); n != 1 {
		t.Errorf("the string was emitted %d times, want 1", n)
	}
}

// A symbol this unit defines and also references has to be one symbol. The
// class object, the ivar offsets and the functions all take the same route
// through lower, and each of them was a duplicate-name failure once.
func TestDefinitionAndReferenceAreOneSymbol(t *testing.T) {
	out, diags := build(t, `
__attribute__((objc_root_class)) @interface K {
    int _n;
}
+ (K *)make;
- (int)n;
@end
@implementation K
+ (K *)make { return 0; }
- (int)n { return _n; }
@end
void twice(void);
void twice(void) { }
void call(void) { twice(); [K make]; }
`)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("lower: %s", d.Message)
		}
	}
	for _, sym := range []string{"@_OBJC_CLASS_$_K", "@_OBJC_IVAR_$_K$_n", "@_twice"} {
		if strings.Contains(out, "import global "+sym+" ") ||
			strings.Contains(out, "import func "+sym+"(") {
			t.Errorf("%s is imported by a module that defines it", sym)
		}
	}
}

// What lower does not do yet, it says so about — once, as an error, naming
// the construct rather than the expression.
func TestUnsupportedIsReportedOncePerConstruct(t *testing.T) {
	_, diags := build(t, `
__attribute__((objc_root_class)) @interface K @end
void f(void) {
    @try { } @catch (id e) { }
    @try { } @catch (id e) { }
}
`)
	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "@try") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the same unsupported construct was reported %d times, want 1", n)
	}
}

// A builder failure inside ir surfaces as exactly one diagnostic: every
// call after the first is a no-op, and reporting each would be a cascade.
//
// The module that comes back cannot be formatted, which is the point — the
// text format will not print what it cannot read back — so this calls Lower
// directly rather than going through build.
func TestBuilderFailureIsOneDiagnostic(t *testing.T) {
	src := "int x; int y; int f(void) { return x + y; }"
	f := token.NewFile("t.m", []byte(src))
	file, _ := parser.ParseFile(f, 0)
	info, _ := analyzer.Check(f, file, types.LP64(), 0)
	// A module name that is not an identifier fails at construction, and
	// every builder call after it is a no-op.
	_, diags := lower.Lower(f, file, info, lower.Options{
		Name: "not an ident", Target: ir.AArch64MacOS, Model: types.LP64(),
		ABI: runtime.Darwin64(), Arch: runtime.ARM64, SymbolPrefix: "_",
	})
	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "internal:") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("got %d internal diagnostics, want 1: %v", n, diags)
	}
}

// Diagnostics come back sorted, so that a caller printing them walks the
// file in order.
func TestDiagnosticsAreSorted(t *testing.T) {
	_, diags := build(t, `
void a(void) { __asm__("nop"); }
void b(void) { int (^x)(void) = ^{ return 1; }; (void)x; }
`)
	for i := 1; i < len(diags); i++ {
		if diags[i].Pos < diags[i-1].Pos {
			t.Fatalf("diagnostic %d is out of order", i)
		}
	}
}
