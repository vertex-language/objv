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
	opt := lower.Options{
		Name: "t", Target: ir.AArch64MacOS, Model: types.LP64(),
		ABI: runtime.Darwin64(), Arch: runtime.ARM64, SymbolPrefix: "_",
	}
	for _, o := range opts {
		o(&opt)
	}
	// The analyzer runs in the mode lower will: ARC's ownership is what
	// analysis inferred, and lowering it against a tree analyzed without it
	// would be lowering something nobody checked.
	var mode analyzer.Mode
	if opt.ARC {
		mode |= analyzer.ARC
	}
	info, _ := analyzer.Check(f, file, types.LP64(), mode)
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
void f(void) {
    __asm__("nop");
    __asm__("nop");
}
`)
	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "inline assembly") {
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

// A header's `extern int counter;` and the definition below it are one
// object, and whichever declaration is read first must not decide whether
// the module defines the name or imports it. Reading the extern first and
// then skipping the definition leaves the object undefined — which links
// only if some other unit happens to define it.
func TestExternThenDefinition(t *testing.T) {
	out, diags := build(t, "extern int counter;\nint counter = 7;\n")
	for _, d := range diags {
		t.Errorf("%v", d.Message)
	}
	mustContain(t, out, "global rw @_counter")
	if strings.Contains(out, "import global @_counter") {
		t.Error("the object was imported by the module that defines it")
	}
}

// §6.9.2's tentative definition is a definition too: `int a;` at file scope
// with no initializer defines the object, and a unit with both that and an
// extern declaration still defines one.
func TestTentativeDefinition(t *testing.T) {
	out, _ := build(t, "extern int a;\nint a;\n")
	mustContain(t, out, "global rw @_a")
	if strings.Contains(out, "import global @_a") {
		t.Error("a tentative definition was imported")
	}
}

// ---- automatic reference counting ----

// arc lowers a snippet with ARC on, in both phases.
func arcBuild(t *testing.T, src string) string {
	t.Helper()
	out, diags := build(t, arcPrelude+src, func(o *lower.Options) { o.ARC = true })
	for _, d := range diags {
		t.Errorf("%s", d.Message)
	}
	return out
}

const arcPrelude = `
__attribute__((objc_root_class)) @interface NSObject
- (instancetype)init;
+ (instancetype)alloc;
- (id)copy;
@end
@interface NSString : NSObject
+ (instancetype)stringWithUTF8String:(const char *)s;
@end
`

// §5.6's default: a local that holds an object keeps it alive for as long as
// it is in scope, and lets it go where the scope ends.
func TestARCStrongLocal(t *testing.T) {
	out := arcBuild(t, `
	void f(void) {
		NSString *s = [NSString stringWithUTF8String:"x"];
		(void)s;
	}`)
	mustContain(t, out, "call @_objc_retain", "call @_objc_release")
}

// A method in one of the retaining families hands back an object the caller
// owns, so `[[NSObject alloc] init]` is one +1 and not two: init consumes the
// receiver it was given. Nothing retains it, and the variable's scope
// releases it once.
func TestARCAllocInitIsOneRetain(t *testing.T) {
	out := arcBuild(t, `
	void f(void) {
		NSObject *o = [[NSObject alloc] init];
		(void)o;
	}`)
	if n := strings.Count(out, "call @_objc_retain("); n != 0 {
		t.Errorf("%d retains for an alloc/init, want none\n%s", n, out)
	}
	if n := strings.Count(out, "call @_objc_release("); n != 1 {
		t.Errorf("%d releases, want one\n%s", n, out)
	}
}

// An object the statement produced at +1 and nothing took is released where
// the statement ends.
func TestARCUnusedTemporaryIsReleased(t *testing.T) {
	out := arcBuild(t, `
	void use(NSObject *o);
	void f(void) { use([[NSObject alloc] init]); }`)
	mustContain(t, out, "call @_objc_release")
}

// `self = [super init]` takes the +1 its superclass produced rather than
// releasing it, and `return self` hands that same one back.
func TestARCInitializerTransfersSelf(t *testing.T) {
	out := arcBuild(t, `
	@interface Box : NSObject
	@end
	@implementation Box
	- (instancetype)init { self = [super init]; return self; }
	@end`)
	if strings.Contains(out, "call @_objc_release") {
		t.Errorf("an initializer released the object it was handed\n%s", out)
	}
	if strings.Contains(out, "call @_objc_retain(") {
		t.Errorf("an initializer retained what it already owned\n%s", out)
	}
}

// A class with __strong instance variables owes the runtime a destructor,
// and the flag word has to say so: objc4 checks the flag before it looks the
// selector up.
func TestARCCxxDestruct(t *testing.T) {
	out := arcBuild(t, `
	@interface Box : NSObject { NSObject *_held; }
	@end
	@implementation Box
	@end`)
	mustContain(t, out, "@__i_Box__$cxx_destruct", "call @_objc_storeStrong")
	// RO_HAS_CXX_STRUCTORS | RO_IS_ARC | RO_HAS_CXX_DTOR_ONLY.
	mustContain(t, out, "{ 388,")
}

// Without ARC none of it is emitted: the ownership is the program's.
func TestNoARCEmitsNothing(t *testing.T) {
	out, _ := build(t, arcPrelude+`
	void f(void) {
		NSObject *o = [[NSObject alloc] init];
		(void)o;
	}`)
	for _, call := range []string{"objc_retain", "objc_release", "objc_storeStrong"} {
		if strings.Contains(out, call) {
			t.Errorf("manual reference counting emitted %s\n%s", call, out)
		}
	}
}

// A struct-valued property's accessors travel the way every other aggregate
// does: the result comes back through storage the caller supplied, and the
// argument arrives as a pointer to the caller's copy. A send described
// without those attributes passes a pointer where the method reads
// registers, which compiles and returns the wrong struct.
func TestStructPropertyAccessors(t *testing.T) {
	out, diags := build(t, `
__attribute__((objc_root_class)) @interface NSObject @end
typedef struct { double x, y; } Pt;
@interface Framed : NSObject
@property (nonatomic, assign) Pt origin;
@end
@implementation Framed
@end
double readIt(Framed *f) { return f.origin.x; }
void writeIt(Framed *f, Pt p) { f.origin = p; }
`)
	for _, d := range diags {
		t.Errorf("%s", d.Message)
	}
	mustContain(t,
		out,
		"@__i_Framed__origin(%__ret ptr sret",
		"@__i_Framed__setOrigin_(%self ptr, %_cmd ptr, %value ptr byval",
		"_sret_", // the send's own type says so too
		"_byval_",
	)
}
