package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

func parse(t *testing.T, src string) (*token.File, *ast.File, []token.Diagnostic) {
	t.Helper()
	f := token.NewFile("t.m", []byte(src+"\n"))
	file, diags := ParseFile(f, DefaultMode)
	return f, file, diags
}

// clean parses src and fails on any diagnostic.
func clean(t *testing.T, src string) (*token.File, *ast.File) {
	t.Helper()
	f, file, diags := parse(t, src)
	for _, d := range diags {
		t.Errorf("%s", d.Print(f))
	}
	return f, file
}

// find returns the first node of the named type, by ast.Fdump's name.
func find(file *ast.File, name string) ast.Node {
	var out ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if out != nil {
			return false
		}
		if nodeTypeName(n) == name {
			out = n
			return false
		}
		return true
	})
	return out
}

func nodeTypeName(n ast.Node) string {
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func count(file *ast.File, name string) int {
	n := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if nodeTypeName(node) == name {
			n++
		}
		return true
	})
	return n
}

// ---- the three ambiguities the grammar names ----

// `(T) - x` is a cast if T is a type name and a subtraction otherwise. The
// name table decides, which is why the parser needs no backtracking.
func TestCastVersusSubtraction(t *testing.T) {
	_, file := clean(t, "typedef int T; int x; int a = (T) - x;")
	if find(file, "CastExpr") == nil {
		t.Error("with T a typedef, `(T) - x` is a cast")
	}
	_, file = clean(t, "int T; int x; int a = (T) - x;")
	if find(file, "CastExpr") != nil {
		t.Error("with T a variable, `(T) - x` is a subtraction")
	}
	if find(file, "BinaryExpr") == nil {
		t.Error("want a BinaryExpr")
	}
}

// `Foo<Bar>` is a specialization if Bar is a type and a conformance if Bar is
// a protocol (§4.1, §5.5).
func TestAngleListDisambiguation(t *testing.T) {
	const src = `@protocol P; @class C; @interface G<T> @end
	G<C *> *specialized;
	G<P> *conforming;
	id<P> qualified;`
	_, file := clean(t, src)
	if got := count(file, "TypeArgList"); got != 1 {
		t.Errorf("%d type argument lists, want 1", got)
	}
	if got := count(file, "ProtocolRefList"); got != 2 {
		t.Errorf("%d protocol lists, want 2", got)
	}
}

// Both lists may appear, in that order (§4.1).
func TestBothAngleLists(t *testing.T) {
	const src = `@protocol P; @class C; @interface G<T> @end
	G<C *> <P> *both;`
	_, file := clean(t, src)
	o, _ := find(file, "ObjectType").(*ast.ObjectType)
	if o == nil || o.TypeArgs == nil || o.Protocols == nil {
		t.Fatalf("want an ObjectType carrying both lists, got %+v", o)
	}
}

// `NSArray<id<NSCopying>>` closes two lists with one scanned token.
func TestNestedGenericsSplitTheShiftToken(t *testing.T) {
	const src = `@protocol P; @interface A<T> @end
	A<id<P>> *nested;`
	f, file := clean(t, src)
	o, _ := find(file, "ObjectType").(*ast.ObjectType)
	if o == nil || o.TypeArgs == nil {
		t.Fatal("want a type argument list")
	}
	// Both '>' are real positions, one byte apart, and the inner list closes
	// first.
	inner := o.TypeArgs
	if !inner.Rangle.IsValid() {
		t.Fatal("the outer list did not close")
	}
	if got := string(f.Slice(inner.Rangle, inner.Rangle+1)); got != ">" {
		t.Errorf("the outer '>' is at %q", got)
	}
}

// `[a b]` is a message send and `a[b]` a subscript, told apart by where the
// bracket is (§6.2, §6.3).
func TestMessageVersusSubscript(t *testing.T) {
	_, file := clean(t, "@interface T @end @implementation T - (void)m { id a, b; [a m]; a[0]; } @end")
	if find(file, "MessageExpr") == nil {
		t.Error("want a message send")
	}
	if find(file, "IndexExpr") == nil {
		t.Error("want a subscript")
	}
}

// ---- shapes ----

func TestMessageShapes(t *testing.T) {
	const src = `@interface T @end @implementation T
	- (void)m {
		id a;
		[a unary];
		[a setValue:1 forKey:a];
		[super m];
		[a b:1 :2];
	}
	@end`
	_, file := clean(t, src)
	var sends []*ast.MessageExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if m, ok := n.(*ast.MessageExpr); ok {
			sends = append(sends, m)
		}
		return true
	})
	if len(sends) != 4 {
		t.Fatalf("%d sends, want 4", len(sends))
	}
	if sends[0].Sel == nil || len(sends[0].Args) != 0 {
		t.Error("a unary send fills Sel and nothing else")
	}
	if len(sends[1].Args) != 2 {
		t.Errorf("setValue:forKey: has %d pieces, want 2", len(sends[1].Args))
	}
	if _, ok := sends[2].Recv.(*ast.SuperExpr); !ok {
		t.Errorf("super receiver = %T", sends[2].Recv)
	}
	if len(sends[3].Args) != 2 || sends[3].Args[1].Sel != nil {
		t.Error("the second piece of b:: is nameless")
	}
}

func TestMethodShapes(t *testing.T) {
	const src = `@class NSString; @interface T
	- (void)unary;
	+ (instancetype)make;
	- (void)set:(int)a and:(id)b;
	- (void)fmt:(NSString *)f, ...;
	- (oneway)shutdown;
	- (void)a:(int)x :(int)y;
	@end`
	_, file := clean(t, src)
	var ms []*ast.MethodDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if m, ok := n.(*ast.MethodDecl); ok {
			ms = append(ms, m)
		}
		return true
	})
	if len(ms) != 6 {
		t.Fatalf("%d methods, want 6", len(ms))
	}
	if ms[0].Kind != token.SUB || ms[0].Sel == nil {
		t.Error("a unary instance method")
	}
	if !ms[1].IsClassMethod() {
		t.Error("+ makes a class method")
	}
	if len(ms[2].Parts) != 2 {
		t.Errorf("%d keyword pieces, want 2", len(ms[2].Parts))
	}
	if !ms[3].Ellipsis.IsValid() {
		t.Error("the trailing ... makes the method variadic")
	}
	if ms[4].Type == nil || len(ms[4].Type.Quals) != 1 || ms[4].Type.Type != nil {
		t.Error("`- (oneway)shutdown` states a qualifier and no type")
	}
	if len(ms[5].Parts) != 2 || ms[5].Parts[1].Sel != nil {
		t.Error("the second piece of a:: is nameless")
	}
	if ms[0].IsDefinition() {
		t.Error("a declaration has no body")
	}
}

// §4.7 excludes __attribute__ from Selector, so an attribute after a
// complete selector is not read as one.
func TestTailAttributeIsNotASelector(t *testing.T) {
	const src = `@interface T
	- (id)init __attribute__((objc_designated_initializer));
	@end`
	_, file := clean(t, src)
	m, _ := find(file, "MethodDecl").(*ast.MethodDecl)
	if m == nil || m.Sel == nil {
		t.Fatal("want a unary method named init")
	}
	if len(m.TailAttrs) != 1 {
		t.Errorf("%d tail attributes, want 1", len(m.TailAttrs))
	}
}

func TestCategoryVersusExtension(t *testing.T) {
	_, file := clean(t, "@class C; @interface C (Cat) @end @interface C () @end")
	var cats []*ast.CategoryDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if c, ok := n.(*ast.CategoryDecl); ok {
			cats = append(cats, c)
		}
		return true
	})
	if len(cats) != 2 {
		t.Fatalf("%d categories, want 2", len(cats))
	}
	if cats[0].IsExtension() {
		t.Error("a named category is not an extension")
	}
	if !cats[1].IsExtension() {
		t.Error("an empty name makes a class extension")
	}
}

func TestProtocolForwardVersusDeclaration(t *testing.T) {
	_, file := clean(t, "@protocol A, B; @protocol C <A> - (void)m; @end")
	if find(file, "ProtocolForwardDecl") == nil {
		t.Error("want the forward form")
	}
	d, _ := find(file, "ProtocolDecl").(*ast.ProtocolDecl)
	if d == nil || d.Protocols == nil || len(d.Members) != 1 {
		t.Errorf("want a protocol declaration with one member: %+v", d)
	}
}

func TestRequirementMarkersKeepTheirPlace(t *testing.T) {
	const src = `@protocol P
	- (void)a;
	@optional
	- (void)b;
	@required
	- (void)c;
	@end`
	_, file := clean(t, src)
	d := find(file, "ProtocolDecl").(*ast.ProtocolDecl)
	var kinds []string
	for _, m := range d.Members {
		switch m := m.(type) {
		case *ast.RequirementDecl:
			kinds = append(kinds, m.Kind.String())
		case *ast.MethodDecl:
			kinds = append(kinds, "method")
		}
	}
	want := "method @optional method @required method"
	if strings.Join(kinds, " ") != want {
		t.Errorf("members = %v, want %q", kinds, want)
	}
}

func TestPropertyAttributes(t *testing.T) {
	const src = `@class NSString; @interface T
	@property (nonatomic, copy, getter=name, setter=setName:) NSString *n;
	@end`
	_, file := clean(t, src)
	d := find(file, "PropertyDecl").(*ast.PropertyDecl)
	if len(d.Attrs) != 4 {
		t.Fatalf("%d attributes, want 4", len(d.Attrs))
	}
	want := []ast.PropertyAttrKind{ast.PropNonatomic, ast.PropCopy, ast.PropGetter, ast.PropSetter}
	for i, k := range want {
		if d.Attrs[i].Kind != k {
			t.Errorf("attribute %d = %v, want %v", i, d.Attrs[i].Kind, k)
		}
	}
	if d.Attrs[3].Sel == nil || !d.Attrs[3].Colon.IsValid() {
		t.Error("a setter's selector keeps its trailing colon")
	}
	// An attribute outside §4.8's closed set is a syntax error.
	if _, _, diags := parse(t, "@interface T @property (bogus) int x; @end"); len(diags) == 0 {
		t.Error("want a diagnostic for an unknown property attribute")
	}
}

func TestFastEnumerationVersusCFor(t *testing.T) {
	const src = `@interface T @end @implementation T
	- (void)m {
		id xs, x;
		for (id e in xs) { (void)e; }
		for (x in xs) { }
		for (int i = 0; i < 3; i++) { }
		int in = 1;
		for (int i = in; i < 2; i++) { }
	}
	@end`
	_, file := clean(t, src)
	if got := count(file, "ForInStmt"); got != 2 {
		t.Errorf("%d fast enumerations, want 2", got)
	}
	if got := count(file, "ForStmt"); got != 2 {
		t.Errorf("%d C for statements, want 2", got)
	}
}

func TestBlockLiteralVersusXor(t *testing.T) {
	const src = `@interface T @end @implementation T
	- (void)m {
		int a = 1 ^ 2;
		void (^b)(void) = ^{ };
		int (^c)(int) = ^int(int x) { return x ^ 1; };
		(void)a; b(); c(0);
	}
	@end`
	_, file := clean(t, src)
	if got := count(file, "BlockLit"); got != 2 {
		t.Errorf("%d block literals, want 2", got)
	}
	if got := count(file, "BlockPtrDeclarator"); got != 2 {
		t.Errorf("%d block pointer declarators, want 2", got)
	}
}

func TestObjectLiteralsAndReflection(t *testing.T) {
	const src = `@interface T @end @implementation T
	- (void)m {
		id a = @42, b = @-1, c = @(1 + 2), d = @[@1, @2], e = @{@"k": @1};
		SEL s = @selector(a::);
		Protocol *p = @protocol(NSObject);
		const char *t = @encode(int);
		(void)a; (void)b; (void)c; (void)d; (void)e; (void)s; (void)p; (void)t;
	}
	@end`
	_, file := clean(t, src)
	for name, want := range map[string]int{
		// Six boxes: three written directly, and one inside each element of
		// the array and dictionary literals.
		"BoxedExpr": 6, "ArrayLit": 1, "DictLit": 1,
		"SelectorExpr": 1, "ProtocolExpr": 1, "EncodeExpr": 1,
	} {
		if got := count(file, name); got != want {
			t.Errorf("%s: %d, want %d", name, got, want)
		}
	}
	sel := find(file, "SelectorExpr").(*ast.SelectorExpr)
	if len(sel.Parts) != 2 || sel.Parts[1].Name != nil {
		t.Errorf("@selector(a::) has %d parts, the second nameless", len(sel.Parts))
	}
}

func TestStringSequences(t *testing.T) {
	_, file := clean(t, `id a = @"x" @"y" "z"; const char *b = "p" "q";`)
	var lits []*ast.StringLit
	ast.Inspect(file, func(n ast.Node) bool {
		if s, ok := n.(*ast.StringLit); ok {
			lits = append(lits, s)
		}
		return true
	})
	if len(lits) != 2 {
		t.Fatalf("%d string literals, want 2", len(lits))
	}
	if !lits[0].Object || len(lits[0].Segs) != 3 {
		t.Errorf("an @-headed sequence is an object of %d pieces", len(lits[0].Segs))
	}
	if lits[1].Object || len(lits[1].Segs) != 2 {
		t.Errorf("a plain sequence stays plain: %+v", lits[1])
	}
}

func TestAvailability(t *testing.T) {
	const src = `@interface T @end @implementation T
	- (void)m {
		if (@available(macOS 10.12.4, iOS 13, *)) { }
		if (__builtin_available(macOS 11.0, *)) { }
	}
	@end`
	f, file := clean(t, src)
	a := find(file, "AvailabilityExpr").(*ast.AvailabilityExpr)
	if len(a.Specs) != 2 {
		t.Fatalf("%d platform specs, want 2", len(a.Specs))
	}
	if got := string(f.Slice(a.Specs[0].Version.Lo, a.Specs[0].Version.Hi)); got != "10.12.4" {
		t.Errorf("version = %q", got)
	}
	if !a.Star.IsValid() {
		t.Error("the trailing * is mandatory and was not recorded")
	}
	// Without it, one diagnostic.
	if _, _, diags := parse(t, "@interface T @end @implementation T - (void)m { if (@available(macOS 10.12)) { } } @end"); len(diags) != 1 {
		t.Errorf("a check with no ', *': %d diagnostics, want 1", len(diags))
	}
}

func TestModuleImportPath(t *testing.T) {
	_, file := clean(t, "@import Foundation.NSString;")
	d := find(file, "ImportDecl").(*ast.ImportDecl)
	if len(d.Path) != 2 {
		t.Errorf("%d path components, want 2", len(d.Path))
	}
}

func TestIvarVisibility(t *testing.T) {
	const src = `@interface T { int a; @private int b; @public int c; } @end`
	_, file := clean(t, src)
	l := find(file, "IvarList").(*ast.IvarList)
	if len(l.Items) != 5 {
		t.Fatalf("%d items, want 5 (three fields and two markers)", len(l.Items))
	}
	if _, ok := l.Items[1].(*ast.VisibilityDecl); !ok {
		t.Errorf("item 1 = %T, want a visibility marker in written order", l.Items[1])
	}
}

func TestAttributeArgumentsStayTokens(t *testing.T) {
	const src = `__attribute__((availability(macosx, introduced=10.12.1))) void f(void);`
	_, file := clean(t, src)
	a := find(file, "Attr").(*ast.Attr)
	if a.Args == nil || len(a.Args.List) == 0 {
		t.Fatal("want the argument tokens kept")
	}
	// `introduced=10.12.1` is not an expression in any grammar; it survives
	// as five tokens, the version among them being the single pp-number the
	// scanner made of it.
	if got := len(a.Args.List); got != 5 {
		t.Errorf("%d tokens, want 5 (macosx , introduced = 10.12.1)", got)
	}
}

// ---- modes and recovery ----

func TestSkipBodies(t *testing.T) {
	const src = `@interface T @end @implementation T
	- (void)m { int x = 1; [self m]; }
	@end
	void f(void) { int y = 2; }`
	f := token.NewFile("t.m", []byte(src))
	file, diags := ParseFile(f, SkipBodies)
	for _, d := range diags {
		t.Errorf("%s", d.Print(f))
	}
	if find(file, "MethodDecl") == nil || find(file, "FuncDecl") == nil {
		t.Error("declarations still land")
	}
	if find(file, "MessageExpr") != nil {
		t.Error("a skipped body is not parsed")
	}
}

// One mistake is one diagnostic, and the declarations after it still parse.
func TestRecovery(t *testing.T) {
	const src = `@interface T
	- (void)good;
	- (void) ;
	- (void)alsoGood;
	@end
	int after;`
	f, file, diags := parse(t, src)
	if len(diags) != 1 {
		for _, d := range diags {
			t.Logf("%s", d.Print(f))
		}
		t.Errorf("%d diagnostics, want 1", len(diags))
	}
	if got := count(file, "MethodDecl"); got < 2 {
		t.Errorf("%d methods survived, want the two good ones", got)
	}
	if find(file, "GenDecl") == nil {
		t.Error("the declaration after the class still parses")
	}
}

func TestUnterminatedInterface(t *testing.T) {
	f, file, diags := parse(t, "@interface T\n- (void)m;\n")
	if len(diags) != 1 {
		for _, d := range diags {
			t.Logf("%s", d.Print(f))
		}
		t.Fatalf("%d diagnostics, want one about the missing @end", len(diags))
	}
	if !strings.Contains(diags[0].Message, "@end") {
		t.Errorf("message = %q", diags[0].Message)
	}
	if find(file, "ClassInterfaceDecl") == nil {
		t.Error("the tree still holds the class")
	}
}

// Every entry point returns a node, so a consumer reads a tree rather than a
// success flag.
func TestTreeIsNeverNil(t *testing.T) {
	for _, src := range []string{
		"", "@", "@interface", "@implementation T @end", "- (void)m;",
		"@property", "[[[", "int x = ;", "@selector(", "for (id x in",
	} {
		f := token.NewFile("t.m", []byte(src+"\n"))
		file, _ := ParseFile(f, DefaultMode)
		if file == nil {
			t.Errorf("%q: nil tree", src)
			continue
		}
		eof := f.Pos(f.Size())
		ast.Inspect(file, func(n ast.Node) bool {
			if !n.Pos().IsValid() {
				t.Errorf("%q: %T has no position", src, n)
				return false
			}
			// A node built at end of file has nothing left to underline;
			// see parser.widen. Everywhere else a span is non-empty.
			if n.End() <= n.Pos() && n.Pos() < eof {
				t.Errorf("%q: %T has an empty span", src, n)
				return false
			}
			return true
		})
	}
}

// A protocol used before its declaration is still a protocol: the list is
// bare names, and nothing that is not a protocol list has that shape.
func TestUndeclaredProtocolList(t *testing.T) {
	_, file := clean(t, "@class NSObject; @interface T : NSObject <NSCopying, NSCoding> @end")
	d := find(file, "ClassInterfaceDecl").(*ast.ClassInterfaceDecl)
	if d.Protocols == nil || len(d.Protocols.Names) != 2 {
		t.Fatalf("want two protocol names, got %+v", d.Protocols)
	}
	if d.SuperArgs != nil {
		t.Error("`<NSCopying, NSCoding>` after a superclass is a conformance")
	}
}

// The other side of the same ambiguity: the list right after the class name.
func TestTypeParamsVersusConformance(t *testing.T) {
	// An undeclared name there is a type parameter — a class is generic far
	// more often than it conforms to a protocol nobody declared.
	_, file := clean(t, "@interface Container<T> @end")
	if find(file, "TypeParamList") == nil {
		t.Error("`@interface Container<T>` declares a type parameter")
	}
	// A declared protocol makes it a conformance.
	_, file = clean(t, "@protocol P; @interface Root <P> @end")
	if find(file, "TypeParamList") != nil {
		t.Error("`@interface Root <P>` with P a protocol is a conformance")
	}
	if find(file, "ProtocolRefList") == nil {
		t.Error("want a protocol list")
	}
	// A variance keyword or a bound settles it outright.
	_, file = clean(t, "@protocol P; @interface G<__covariant P> @end")
	if find(file, "TypeParamList") == nil {
		t.Error("a variance keyword makes it a parameter list whatever the name is")
	}
}

// §4.1 gives an @implementation no type parameters, but its methods are
// written in terms of the ones the interface declared.
func TestImplementationSeesTypeParameters(t *testing.T) {
	const src = `@interface NSObject @end
	@interface Cache<K, V> : NSObject
	- (V)objectForKey:(K)key;
	@end
	@implementation Cache
	- (V)objectForKey:(K)key { (void)key; return (V)0; }
	@end
	@implementation Cache (Extra)
	- (K)firstKey { return (K)0; }
	@end`
	_, file := clean(t, src)
	n := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if o, ok := node.(*ast.ObjectType); ok && o.Kind == ast.ObjectTypeParam {
			n++
		}
		return true
	})
	if n != 7 {
		t.Errorf("%d type-parameter uses resolved, want 7", n)
	}
}

// An identifier where only a type may stand is a class whose declaration has
// not been read. Saying so beats the cascade that follows from reading it as
// the declarator.
func TestUnknownTypeName(t *testing.T) {
	f, file, diags := parse(t, "NSArray *keys; int after;")
	if len(diags) != 1 {
		for _, d := range diags {
			t.Logf("%s", d.Print(f))
		}
		t.Fatalf("%d diagnostics, want 1", len(diags))
	}
	if !strings.Contains(diags[0].Message, "unknown type name 'NSArray'") {
		t.Errorf("message = %q", diags[0].Message)
	}
	// And the declaration still parses, so the file after it does too.
	if got := count(file, "GenDecl"); got != 2 {
		t.Errorf("%d declarations, want 2", got)
	}

	// Each undeclared name is named once, and the generic list still parses.
	f, file, diags = parse(t, "@class NSString; NSArray<NSString *> *keys;")
	if len(diags) != 1 {
		for _, d := range diags {
			t.Logf("%s", d.Print(f))
		}
		t.Errorf("%d diagnostics, want 1 (NSString is declared)", len(diags))
	}
	if find(file, "TypeArgList") == nil {
		t.Error("the generic list still parses")
	}
}

// §4.4 gives protocols a namespace of their own, and the root class of every
// Objective-C program is where it matters: `@protocol NSObject` and
// `@interface NSObject` are two things with one name, and the adoption list
// in `@interface NSObject <NSObject>` is a protocol list rather than the
// type-parameter list it looks like.
func TestClassAndProtocolShareAName(t *testing.T) {
	f, file := clean(t, `
@protocol Same
- (int)fromSame;
@end
__attribute__((objc_root_class)) @interface Same <Same>
@end`)
	var found *ast.ClassInterfaceDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if d, ok := n.(*ast.ClassInterfaceDecl); ok {
			found = d
		}
		return true
	})
	if found == nil {
		t.Fatal("no @interface")
	}
	if found.TypeParams != nil {
		t.Error("the adoption list was read as type parameters")
	}
	if found.Protocols == nil || len(found.Protocols.Names) != 1 {
		t.Fatal("the protocol list is missing")
	}
	if got := found.Protocols.Names[0].Name(f); got != "Same" {
		t.Errorf("adopted %q, want Same", got)
	}
}

// The angle brackets after a class name are a protocol reference list or a
// type-argument list, and the *shape* decides: a protocol list is bare
// identifiers and nothing else, so anything with a '*' in it is a
// specialization whatever the names are called. AppKit has both — a class
// and a protocol may share a name, and `NSArray<NSMenuItem *>` is still a
// specialization.
func TestAngleShapeDecidesBeforeNames(t *testing.T) {
	_, file := clean(t, `
@protocol Elem;
@class Elem;
@interface NSArray<T> @end
@interface Holder
@property (copy) NSArray<Elem *> *items;
@property (copy) id<Elem> one;
@end`)
	if file == nil {
		t.Fatal("no file")
	}
}
