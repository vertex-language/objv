package analyzer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

const prelude = `
__attribute__((objc_root_class))
@interface NSObject
- (instancetype)init;
+ (instancetype)alloc;
@end
@protocol NSCopying
- (id)copyWithZone:(void *)zone;
@end
@interface NSString : NSObject <NSCopying>
- (unsigned long)length;
@end
@interface NSNumber : NSObject
@end
@interface NSArray<ObjectType> : NSObject
- (ObjectType)objectAtIndexedSubscript:(unsigned long)i;
@end
`

func check(t *testing.T, mode analyzer.Mode, src string) (*token.File, *ast.File, *analyzer.Info, []token.Diagnostic) {
	t.Helper()
	f := token.NewFile("t.m", []byte(prelude+src+"\n"))
	file, diags := parser.ParseFile(f, 0)
	for _, d := range diags {
		t.Fatalf("parse: %s", d.Print(f))
	}
	info, ds := analyzer.Check(f, file, types.LP64(), mode)
	return f, file, info, ds
}

// errs returns the error messages, and fails on none where some were wanted.
func errs(ds []token.Diagnostic) []string {
	var out []string
	for _, d := range ds {
		if d.Severity == token.Error {
			out = append(out, d.Message)
		}
	}
	return out
}

func clean(t *testing.T, mode analyzer.Mode, src string) (*token.File, *ast.File, *analyzer.Info) {
	t.Helper()
	f, file, info, ds := check(t, mode, src)
	for _, d := range ds {
		if d.Severity == token.Error {
			t.Errorf("%s", d.Print(f))
		}
	}
	return f, file, info
}

func wantError(t *testing.T, mode analyzer.Mode, src, want string) {
	t.Helper()
	_, _, _, ds := check(t, mode, src)
	for _, m := range errs(ds) {
		if strings.Contains(m, want) {
			return
		}
	}
	t.Errorf("want an error containing %q, got %v", want, errs(ds))
}

// ---- what analysis records ----

func TestInfoRecordsTheHierarchy(t *testing.T) {
	_, _, info := clean(t, 0, `
	@interface Cache : NSObject
	@property (nonatomic, copy) NSString *name;
	- (void)ping;
	@end
	@implementation Cache
	- (void)ping { }
	@end`)

	var cache *types.Class
	for _, k := range info.Classes {
		if k.Name == "Cache" {
			cache = k
		}
	}
	if cache == nil {
		t.Fatal("Cache is not in Info.Classes")
	}
	if cache.Super == nil || cache.Super.Name != "NSObject" {
		t.Errorf("superclass = %v", cache.Super)
	}
	// A property brings a backing variable and two accessors with it.
	if iv, _ := cache.FindIvar("_name"); iv == nil {
		t.Error("the property was not synthesized")
	} else if !iv.Synthesized {
		t.Error("the backing variable should be marked synthesized")
	}
	for _, sel := range []string{"name", "setName:", "ping"} {
		if cache.Lookup(sel, false) == nil {
			t.Errorf("no method '%s'", sel)
		}
	}
	// Every selector the unit mentions is recorded once.
	seen := map[string]int{}
	for _, s := range info.Selectors {
		seen[s]++
	}
	for s, n := range seen {
		if n != 1 {
			t.Errorf("selector %q recorded %d times", s, n)
		}
	}
	if seen["setName:"] == 0 {
		t.Error("a property's setter is a selector the unit mentions")
	}
}

func TestInfoRecordsSendsAndProperties(t *testing.T) {
	_, file, info := clean(t, 0, `
	@interface Cache : NSObject
	@property (nonatomic, copy) NSString *name;
	@end
	@implementation Cache
	- (void)use {
		unsigned long n = [self.name length];
		(void)n;
	}
	@end`)

	sends, props := 0, 0
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.MessageExpr:
			m, ok := info.Sends[n]
			if !ok {
				t.Error("a send was not recorded")
			}
			if m != nil && m.Sel == "length" {
				sends++
			}
		case *ast.MemberExpr:
			if p := info.Props[n]; p != nil && p.Name == "name" {
				props++
			}
		}
		return true
	})
	if sends != 1 {
		t.Errorf("%d resolved sends of 'length', want 1", sends)
	}
	if props != 1 {
		t.Errorf("%d dot-syntax property accesses recorded, want 1", props)
	}
}

// instancetype follows the receiver, which is what makes an initializer
// chain keep its type.
func TestInstancetype(t *testing.T) {
	_, _, _ = clean(t, 0, `
	@interface Sub : NSString
	- (void)only;
	@end
	@implementation Sub
	- (void)only {
		Sub *s = [[Sub alloc] init];
		[s only];
	}
	@end`)

	wantError(t, 0, `
	@interface Sub : NSString
	@end
	@implementation Sub
	- (void)use {
		NSNumber *wrong = [[Sub alloc] init];
		(void)wrong;
	}
	@end`, "the classes are unrelated")
}

// §5.5: a specialized receiver decides what its methods' type parameters
// mean.
func TestGenericSubstitution(t *testing.T) {
	clean(t, 0, `
	@interface Use : NSObject
	@end
	@implementation Use
	- (void)m {
		NSArray<NSString *> *a;
		NSString *s = a[0];
		unsigned long n = [s length];
		(void)n;
	}
	@end`)

	wantError(t, 0, `
	@interface Use : NSObject
	@end
	@implementation Use
	- (void)m {
		NSArray<NSString *> *a;
		NSNumber *n = a[0];
		(void)n;
	}
	@end`, "the classes are unrelated")
}

// ---- ARC ----

func TestARCOwnershipDefaults(t *testing.T) {
	f, file, info := clean(t, analyzer.ARC, `
	@interface Own : NSObject
	@end
	@implementation Own
	- (void)m {
		NSString *strong;
		__weak NSString *weak;
		int plain;
		(void)strong; (void)weak; (void)plain;
	}
	@end`)

	found := map[string]types.Lifetime{}
	ast.Inspect(file, func(n ast.Node) bool {
		d, ok := n.(*ast.InitDeclarator)
		if !ok {
			return true
		}
		if id := d.Decl.DeclName(); id != nil {
			found[id.Name(f)] = types.LifetimeOf(info.Types[d])
		}
		return true
	})
	if found["strong"] != types.LifeStrong {
		t.Errorf("an object variable defaults to __strong, got %v", found["strong"])
	}
	if found["weak"] != types.LifeWeak {
		t.Errorf("__weak as written, got %v", found["weak"])
	}
	if found["plain"] != types.LifeNone {
		t.Errorf("a non-object has no ownership, got %v", found["plain"])
	}

	// Without ARC there is no ownership to infer.
	f, file, info = clean(t, 0, `
	@interface Own : NSObject
	@end
	@implementation Own
	- (void)m { NSString *s; (void)s; }
	@end`)
	ast.Inspect(file, func(n ast.Node) bool {
		if d, ok := n.(*ast.InitDeclarator); ok {
			if types.LifetimeOf(info.Types[d]) != types.LifeNone {
				t.Error("manual reference counting infers no ownership")
			}
		}
		return true
	})
}

func TestARCIsAMode(t *testing.T) {
	const src = `
	@interface M : NSObject
	@end
	@implementation M
	- (void)m { void *p = (void *)self; (void)p; }
	@end`
	if _, _, _, ds := check(t, 0, src); len(errs(ds)) != 0 {
		t.Errorf("without ARC a plain cast is a plain cast: %v", errs(ds))
	}
	wantError(t, analyzer.ARC, src, "requires a bridge cast under ARC")
}

// ---- the C substrate still works ----

func TestCSubstrate(t *testing.T) {
	clean(t, 0, `
	struct Point { int x, y; };
	enum Color : long { Red, Green };
	typedef int Integer;
	static int add(int a, int b) { return a + b; }
	int use(void) {
		struct Point p = {1, 2};
		Integer i = add(p.x, p.y);
		enum Color c = Green;
		int arr[] = {1, 2, 3};
		_Static_assert(sizeof(arr) == 12, "three ints");
		switch (c) { case Red: break; default: break; }
		return i + arr[0];
	}`)

	for _, c := range []struct{ src, want string }{
		{"int f(void) { return undeclared; }", "'undeclared' is undeclared"},
		{"int f(void) { struct S s; return 0; }", "incomplete type"},
		{"int f(void) { return f(1); }", "too many arguments"},
		{"int f(void) { int a; a(); return 0; }", "not a function"},
		{"int f(void) { const int c = 1; c = 2; return c; }", "cannot assign to a const"},
		{"void f(void) { break; }", "'break' outside"},
		{"void f(void) { goto nowhere; }", "goto to undefined label"},
		{"_Static_assert(0, \"no\");", "static assertion failed"},
	} {
		wantError(t, 0, c.src, c.want)
	}
}

// A tree the parser recovered from must not make the analyzer misbehave: it
// reports what it can and returns.
func TestSyntaxCorpusDoesNotCrash(t *testing.T) {
	files, _ := filepath.Glob("../tests/syntax/*.m")
	if len(files) == 0 {
		t.Skip("no syntax corpus")
	}
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f := token.NewFile(name, src)
		file, _ := parser.ParseFile(f, 0)
		info, _ := analyzer.Check(f, file, types.LP64(), 0)
		if info == nil {
			t.Errorf("%s: nil Info", filepath.Base(name))
		}
	}
}

// Every diagnostic is reported once, and an undeclared name in a loop is one
// mistake however many times it is written.
func TestOneDiagnosticPerMistake(t *testing.T) {
	_, _, _, ds := check(t, 0, `
	int f(void) {
		int total = 0;
		for (int i = 0; i < 10; i++) { total += missing; }
		total += missing;
		return total;
	}`)
	n := 0
	for _, m := range errs(ds) {
		if strings.Contains(m, "'missing' is undeclared") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d diagnostics for one misspelling, want 1", n)
	}
}

// A suffix that is not a suffix is reported where the value is decoded.
//
// §2.3's PPNumber runs through identifier characters, so `1_024` and `10_2`
// are both legal preprocessing tokens; only one of them stands where a value
// is wanted. Apple writes the other inside an availability attribute — whose
// arguments are §8's BalancedTokenSequence and are never decoded — several
// hundred times per compilation, so a scanner that reported it would bury
// every build under it.
func TestIntegerSuffixDiagnostic(t *testing.T) {
	for _, c := range []struct {
		text string
		bad  bool
	}{
		{"1", false}, {"10u", false}, {"5UL", false}, {"2llu", false},
		{"0x1Fu", false}, {"0b1011", false},
		{"1_024", true}, {"4lul", true}, {"5lL", true}, {"10_2", true},
	} {
		var msgs []string
		analyzer.DecodeIntConst(c.text, types.LP64(), func(m string) {
			msgs = append(msgs, m)
		})
		if got := len(msgs) > 0; got != c.bad {
			t.Errorf("%q: %d diagnostics %v, want bad=%v", c.text, len(msgs), msgs, c.bad)
		}
	}
}

// ---- the compiler's own functions ----

// A builtin objv has is typed like any other function, so the expressions
// built on a call to one are typed too. A builtin it does not have is named,
// once: the alternative is an untyped call, and the phases below report the
// operators rather than the cause.
func TestBuiltinsAreTypedOrNamed(t *testing.T) {
	_, _, info := clean(t, 0, `
	double widen(float x) { return __builtin_fabsf(x) + __builtin_sqrt(2.0); }
	int count(unsigned v) { return __builtin_popcount(v) + __builtin_clz(v); }`)
	if len(info.Types) == 0 {
		t.Fatal("nothing was typed")
	}

	wantError(t, 0, `int f(double x) { return __builtin_frobnicate(x); }`,
		"objv does not implement '__builtin_frobnicate'")
}

// Reported once per name, however many times the program writes it: a header
// that defines a dozen functions in terms of one builtin should say one thing.
func TestUnimplementedBuiltinIsReportedOnce(t *testing.T) {
	_, _, _, ds := check(t, 0, `
	int a(void) { return __builtin_frobnicate(1); }
	int b(void) { return __builtin_frobnicate(2); }`)
	n := 0
	for _, m := range errs(ds) {
		if strings.Contains(m, "__builtin_frobnicate") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("reported %d times, want 1: %v", n, errs(ds))
	}
}

// §6.6: __builtin_constant_p is a constant expression, and the analyzer is
// where the answer is decided so that lower cannot give a different one.
func TestConstantPFolds(t *testing.T) {
	_, _, info := clean(t, 0, `
	int yes(void) { return __builtin_constant_p(2 + 3); }
	int no(int x) { return __builtin_constant_p(x); }`)
	got := map[int64]bool{}
	for e, v := range info.Consts {
		if _, ok := e.(*ast.CallExpr); ok {
			got[v] = true
		}
	}
	if !got[1] {
		t.Error("__builtin_constant_p(2 + 3) did not fold to 1")
	}
	if !got[0] {
		t.Error("__builtin_constant_p(x) did not fold to 0")
	}
}

// ---- what a block captures ----

// Capture analysis is a question about scopes, so it is answered here. A
// block captures what its body names and its own scopes do not declare.
func TestBlockCaptures(t *testing.T) {
	_, file, info := clean(t, 0, `
	extern int g;
	int f(int a, int b) {
		static int s = 0;
		int local = a;
		int (^blk)(int) = ^(int p) { return p + local + b + g + s; };
		return blk(1);
	}`)
	got := captureNames(file, info)
	want := []string{"local", "b"}
	if len(got) != len(want) {
		t.Fatalf("captures = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("captures = %v, want %v", got, want)
			break
		}
	}
}

// §6.9 and C's scoping between them: a name the block declares for itself is
// the block's, whatever the enclosing function called its variables.
func TestBlockShadowingIsNotCapture(t *testing.T) {
	_, file, info := clean(t, 0, `
	int f(void) {
		int n = 1;
		int (^blk)(void) = ^{ int n = 2; return n; };
		return blk() + n;
	}`)
	if got := captureNames(file, info); len(got) != 0 {
		t.Errorf("captures = %v, want none", got)
	}
}

// A block inside a block reaches the function's variable through the outer
// one, so both capture it.
func TestNestedBlocksBothCapture(t *testing.T) {
	_, file, info := clean(t, 0, `
	typedef int (^I)(void);
	extern void use(I);
	void f(int n) {
		use(^{ use(^{ return n; }); return n; });
	}`)
	count := 0
	ast.Inspect(file, func(x ast.Node) bool {
		if b, ok := x.(*ast.BlockLit); ok {
			count++
			if names := names(info.Captures[b]); len(names) != 1 || names[0] != "n" {
				t.Errorf("a block captures %v, want [n]", names)
			}
		}
		return true
	})
	if count != 2 {
		t.Fatalf("found %d block literals, want 2", count)
	}
}

// An instance variable inside a block is a reference through self, so what
// the block captures is self.
func TestIvarInBlockCapturesSelf(t *testing.T) {
	_, file, info := clean(t, 0, `
	@interface Box : NSObject { int _n; }
	@end
	@implementation Box
	- (int (^)(void))counter { return ^{ return _n; }; }
	@end`)
	if got := captureNames(file, info); len(got) != 1 || got[0] != "self" {
		t.Errorf("captures = %v, want [self]", got)
	}
}

func captureNames(file *ast.File, info *analyzer.Info) []string {
	var out []string
	ast.Inspect(file, func(x ast.Node) bool {
		if b, ok := x.(*ast.BlockLit); ok {
			out = append(out, names(info.Captures[b])...)
		}
		return true
	})
	return out
}

func names(cs []analyzer.Capture) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

// ---- @available ----

// A clause naming a platform this build is not for is ignored and the check
// succeeds, which is what the trailing `*` means. So a misspelled platform
// does not fail — it quietly makes the check true — and that is the reason
// to report it here.
func TestAvailabilityClausesAreChecked(t *testing.T) {
	clean(t, 0, `int f(void) { if (@available(macOS 10.12.1, iOS 13, *)) return 1; return 0; }`)

	wantError(t, 0, `int f(void) { if (@available(mocOS 12.0, *)) return 1; return 0; }`,
		"'mocOS' is not a platform name")
	wantError(t, 0, `int f(void) { if (@available(macOS 1.2.3.4, *)) return 1; return 0; }`,
		"is not a version")
}

// §4.4 gives protocols a namespace of their own, which matters because the
// root class of every Objective-C program is in it twice: `@protocol
// NSObject` declares -release and -respondsToSelector:, and `@interface
// NSObject <NSObject>` adopts it. A parser that let the class erase the
// protocol read the adoption list as a type-parameter list, and the class
// then declared none of those methods.
func TestClassAdoptsProtocolOfItsOwnName(t *testing.T) {
	clean(t, 0, `
	@protocol Same
	- (int)fromSame;
	@end
	__attribute__((objc_root_class)) @interface Same <Same>
	@end
	int f(Same *s) { return [s fromSame]; }`)
}

// §5.4's instancetype is the receiver's type, and `super` is not a receiver:
// it is self with the lookup starting one class higher. So `self = [super
// init]` in a subclass gives the subclass, which is the whole reason every
// initializer in every Objective-C program is written that way.
func TestSuperInstancetypeIsTheSubclass(t *testing.T) {
	_, file, info := clean(t, 0, `
	@interface Node : NSObject
	- (instancetype)init;
	@end
	@implementation Node
	- (instancetype)init { self = [super init]; return self; }
	@end`)
	var send *ast.MessageExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if m, ok := n.(*ast.MessageExpr); ok && send == nil {
			send = m
		}
		return true
	})
	if send == nil {
		t.Fatal("no send")
	}
	got := info.Types[send]
	o := types.AsObject(got)
	if o == nil || o.Base == nil || o.Base.Name != "Node" {
		t.Errorf("[super init] is %v, want Node *", got)
	}
}
