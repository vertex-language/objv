package types_test

import (
	"strings"
	"testing"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// resolver is the smallest thing that satisfies types.Resolver: enough name
// resolution to build a type, and no more. The analyzer's will do the same
// job against real scopes.
type resolver struct {
	unit     *token.File
	classes  map[string]*types.Class
	protos   map[string]*types.Protocol
	typedefs map[string]types.Type
	reports  []string
}

func (r *resolver) Typedef(id *ast.Ident) types.Type {
	if t, ok := r.typedefs[id.Name(r.unit)]; ok {
		return t
	}
	return types.Typ(types.Int)
}

func (r *resolver) Tag(spec ast.Expr) types.Type {
	switch s := spec.(type) {
	case *ast.StructType:
		name := ""
		if s.Name != nil {
			name = s.Name.Name(r.unit)
		}
		return &types.Record{Name: name, Union: s.Kind == token.UNION, Complete: s.Lbrace.IsValid()}
	case *ast.EnumDecl:
		name := ""
		if s.Name != nil {
			name = s.Name.Name(r.unit)
		}
		return &types.Enum{Name: name, Complete: true, Fixed: s.Base != nil}
	case *ast.TypeName:
		sp := types.BuildSpecs(r.unit, s.Specs, r)
		t, _ := types.BuildDeclarator(r.unit, sp.Type, s.Decl, false, r)
		return t
	case *ast.AtomicType:
		return r.Tag(s.Type)
	}
	return types.Typ(types.Int)
}

func (r *resolver) Object(spec *ast.ObjectType) types.Type {
	o := &types.Object{}
	switch spec.Kind {
	case ast.ObjectID:
	case ast.ObjectClass:
		o.Meta = true
	case ast.ObjectInstancetype:
		o.Instancetype = true
	case ast.ObjectNamed:
		o.Base = r.class(spec.Name.Name(r.unit))
	case ast.ObjectTypeParam:
		return &types.TypeParam{Name: spec.Name.Name(r.unit), Bound: types.ID()}
	}
	if spec.TypeArgs != nil {
		for _, a := range spec.TypeArgs.Args {
			o.Args = append(o.Args, r.Tag(a))
		}
	}
	if spec.Protocols != nil {
		for _, n := range spec.Protocols.Names {
			o.Protocols = append(o.Protocols, r.protocol(n.Name(r.unit)))
		}
	}
	// A class name is an interface type: the star in `NSString *` is the
	// declarator's. id, Class and instancetype are already pointers.
	switch spec.Kind {
	case ast.ObjectNamed:
		return o
	}
	return &types.Pointer{Elem: o}
}

func (r *resolver) class(name string) *types.Class {
	if c, ok := r.classes[name]; ok {
		return c
	}
	c := &types.Class{Name: name}
	r.classes[name] = c
	return c
}

func (r *resolver) protocol(name string) *types.Protocol {
	if p, ok := r.protos[name]; ok {
		return p
	}
	p := &types.Protocol{Name: name}
	r.protos[name] = p
	return p
}

func (r *resolver) Eval(e ast.Expr) (int64, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT_LIT {
		return 0, false
	}
	var n int64
	for _, c := range r.unit.Slice(lit.Lo, lit.Hi) {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	return n, true
}

func (r *resolver) Report(n ast.Node, msg string) { r.reports = append(r.reports, msg) }
func (r *resolver) TypeOf(e ast.Expr) types.Type  { return types.Typ(types.Int) }

// build parses one file of declarations and returns the type of each
// top-level declarator, by name.
func build(t *testing.T, src string) (map[string]types.Type, []string) {
	t.Helper()
	f := token.NewFile("t.m", []byte(src+"\n"))
	file, diags := parser.ParseFile(f, 0)
	for _, d := range diags {
		t.Fatalf("parse: %s", d.Print(f))
	}
	r := &resolver{unit: f,
		classes:  map[string]*types.Class{},
		protos:   map[string]*types.Protocol{},
		typedefs: map[string]types.Type{},
	}
	out := map[string]types.Type{}
	for _, d := range file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		sp := types.BuildSpecs(f, g.Specs, r)
		for _, it := range g.List {
			ty, id := types.BuildDeclarator(f, sp.Type, it.Decl, false, r)
			if id != nil {
				out[id.Name(f)] = ty
				if sp.Storage == token.TYPEDEF {
					r.typedefs[id.Name(f)] = ty
				}
			}
		}
	}
	return out, r.reports
}

func TestBuildC(t *testing.T) {
	got, reports := build(t, `
		int plain;
		int *ptr;
		const int *ptrToConst;
		int *const constPtr;
		int arr[10];
		int matrix[2][3];
		int (*fp)(int, char);
		unsigned long long big;
		struct S { int x; } record;
		typedef int Integer;
		Integer viaTypedef;
	`)
	for name, want := range map[string]string{
		"plain":      "int",
		"ptr":        "int*",
		"ptrToConst": "const int*",
		"constPtr":   "const int*",
		"arr":        "int[10]",
		"matrix":     "int[3][2]",
		"fp":         "int(int, char)*",
		"big":        "unsigned long long",
		"record":     "struct S",
		"viaTypedef": "int",
	} {
		if got[name] == nil {
			t.Errorf("%s: not built", name)
			continue
		}
		if s := got[name].String(); s != want {
			t.Errorf("%s: %s, want %s", name, s, want)
		}
	}
	if len(reports) != 0 {
		t.Errorf("reports: %v", reports)
	}
}

func TestBuildObjC(t *testing.T) {
	got, reports := build(t, `
		@class NSString, NSArray;
		@protocol NSCopying;
		id anyObject;
		Class anyClass;
		NSString *named;
		id<NSCopying> qualified;
		NSArray<NSString *> *specialized;
		__kindof NSString *kindOf;
		__weak id weakRef;
		__strong NSString *strongName;
		NSString *_Nullable maybeName;
		void (^simpleBlock)(void);
		int (^intBlock)(int, int);
		void (^__weak weakBlock)(void);
	`)
	for name, want := range map[string]string{
		"anyObject":   "id",
		"anyClass":    "Class",
		"named":       "NSString*",
		"qualified":   "id<NSCopying>",
		"specialized": "NSArray<NSString*>*",
		"kindOf":      "__kindof NSString*",
		"weakRef":     "__weak id",
		"strongName":  "__strong NSString*",
		"maybeName":   "NSString* _Nullable",
		"simpleBlock": "void(^)(void)",
		"intBlock":    "int(^)(int, int)",
		"weakBlock":   "__weak void(^)(void)",
	} {
		if got[name] == nil {
			t.Errorf("%s: not built", name)
			continue
		}
		if s := got[name].String(); s != want {
			t.Errorf("%s: %s, want %s", name, s, want)
		}
	}
	if len(reports) != 0 {
		t.Errorf("reports: %v", reports)
	}

	// The qualifiers landed on the right level, which is the whole reason
	// an object pointer is two types and not one.
	if types.LifetimeOf(got["weakRef"]) != types.LifeWeak {
		t.Error("__weak belongs to the pointer")
	}
	if o := types.AsObject(got["specialized"]); o == nil || len(o.Args) != 1 {
		t.Error("the type argument belongs to the interface type")
	}
	if !types.IsKindOf(got["kindOf"]) {
		t.Error("__kindof moved to the pointer it qualifies")
	}
	if b := types.AsBlock(got["intBlock"]); b == nil || len(b.Sig.Params) != 2 {
		t.Errorf("block signature = %v", got["intBlock"])
	}
}

// Construction reports the constraints the parser deferred.
func TestBuildReports(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string
	}{
		{"struct S; struct S incomplete[2];", "incomplete element type"},
		{"typedef int F(void); F functions[3];", "array of functions"},
		{"@class NSString; NSString objects[4];", "an array of objects holds pointers"},
		{"restrict int notAPointer;", "restrict requires a pointer"},
		{"__kindof int notAnObject;", "__kindof requires an Objective-C object type"},
		{"typedef static int both;", "multiple storage classes"},
		{"long float wrong;", "invalid type specifier combination"},
	} {
		_, reports := build(t, c.src)
		found := false
		for _, r := range reports {
			if strings.Contains(r, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: reports %v, want one containing %q", c.src, reports, c.want)
		}
	}
}

// An array's element must be complete, and the outermost array of a
// parameter is the one place there is nothing to complete.
func TestParameterAdjustment(t *testing.T) {
	f := token.NewFile("t.m", []byte("struct S; void f(struct S a[], int b[static 4]);\n"))
	file, diags := parser.ParseFile(f, 0)
	for _, d := range diags {
		t.Fatalf("parse: %s", d.Print(f))
	}
	r := &resolver{unit: f, classes: map[string]*types.Class{},
		protos: map[string]*types.Protocol{}, typedefs: map[string]types.Type{}}
	var fn *types.Func
	for _, d := range file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || len(g.List) == 0 {
			continue
		}
		sp := types.BuildSpecs(f, g.Specs, r)
		ty, _ := types.BuildDeclarator(f, sp.Type, g.List[0].Decl, false, r)
		if x, ok := types.Unqualify(ty).(*types.Func); ok {
			fn = x
		}
	}
	if fn == nil {
		t.Fatal("no function type built")
	}
	if len(r.reports) != 0 {
		t.Errorf("reports: %v", r.reports)
	}
	if len(fn.Params) != 2 {
		t.Fatalf("%d parameters", len(fn.Params))
	}
	if got := fn.Params[0].Type.String(); got != "struct S*" {
		t.Errorf("an array parameter is a pointer to the element: %s", got)
	}
	if got := fn.Params[1].Type.String(); got != "int*" {
		t.Errorf("[static 4] adjusts too: %s", got)
	}
}
