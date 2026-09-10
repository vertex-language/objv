package runtime_test

import (
	"testing"

	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// The seam: a class as the front end produces it, encoded as the runtime
// wants it.
//
// Every expectation below came from clang compiling the same source with
// -S and reading the metadata out of the assembly. That is what makes this
// worth having over the unit tests above — those check that this package
// says what clang says about a type built by hand, and this checks that the
// type objv's own front end builds is the same type.
const seamSource = `
__attribute__((objc_root_class))
@interface NSObject @end
@protocol NSCopying @end
@interface NSString : NSObject @end
struct Point { int x; float y; };
@interface Cache : NSObject <NSCopying> {
    int _count;
    NSString *_name;
}
@property (nonatomic, copy) NSString *name;
@property (nonatomic, readonly, getter=isEmpty) int empty;
- (NSString *)objectForKey:(NSString *)key;
- (struct Point)origin;
- (void)move:(struct Point)p by:(int)n;
+ (instancetype)cache;
@end
@implementation Cache
@synthesize name = _name;
- (NSString *)objectForKey:(NSString *)key { return key; }
- (struct Point)origin { struct Point p; p.x = 0; p.y = 0; return p; }
- (void)move:(struct Point)p by:(int)n { (void)p; (void)n; }
+ (instancetype)cache { return (id)0; }
- (int)isEmpty { return _count == 0; }
@end
`

func analyze(t *testing.T) *analyzer.Info {
	t.Helper()
	f := token.NewFile("seam.m", []byte(seamSource))
	file, diags := parser.ParseFile(f, 0)
	for _, d := range diags {
		t.Fatalf("parse: %s", d.Print(f))
	}
	info, ds := analyzer.Check(f, file, types.LP64(), 0)
	for _, d := range ds {
		if d.Severity == token.Error {
			t.Fatalf("check: %s", d.Print(f))
		}
	}
	return info
}

func findClass(info *analyzer.Info, name string) *types.Class {
	for _, k := range info.Classes {
		if k.Name == name {
			return k
		}
	}
	return nil
}

func TestSeamMethodTypes(t *testing.T) {
	info := analyze(t)
	cache := findClass(info, "Cache")
	if cache == nil {
		t.Fatal("Cache was not analyzed")
	}
	model := types.LP64()

	want := map[string]string{
		"objectForKey:": "@24@0:8@16",
		"origin":        "{Point=if}16@0:8",
		"move:by:":      "v28@0:8{Point=if}16i24",
		"cache":         "@16@0:8",
		"isEmpty":       "i16@0:8",
		"name":          "@16@0:8",
		"setName:":      "v24@0:8@16",
	}
	for sel, expect := range want {
		m := cache.Lookup(sel, sel == "cache")
		if m == nil {
			t.Errorf("no method %q on Cache", sel)
			continue
		}
		if got := abi.MethodTypes(m.Ret, m.Params, model); got != expect {
			t.Errorf("%s: %q, want %q", sel, got, expect)
		}
	}
}

func TestSeamIvars(t *testing.T) {
	info := analyze(t)
	cache := findClass(info, "Cache")
	model := types.LP64()

	// The ivar list clang emitted: two entries, with these encodings,
	// alignments (log2) and sizes.
	for _, c := range []struct {
		name  string
		enc   string
		align int64
		size  int64
	}{
		{"_count", "i", 2, 4},
		{"_name", `@"NSString"`, 3, 8},
	} {
		iv, owner := cache.FindIvar(c.name)
		if iv == nil {
			t.Errorf("no ivar %q", c.name)
			continue
		}
		if owner != cache {
			t.Errorf("%s belongs to %s", c.name, owner.Name)
		}
		if got := abi.EncodeExtended(iv.Type, model); got != c.enc {
			t.Errorf("%s type = %q, want %q", c.name, got, c.enc)
		}
		size, _ := model.Sizeof(iv.Type)
		if size != c.size {
			t.Errorf("%s size = %d, want %d", c.name, size, c.size)
		}
		align, _ := model.Alignof(iv.Type)
		if log2(align) != c.align {
			t.Errorf("%s alignment = %d, want log2 %d", c.name, log2(align), c.align)
		}
		if got := runtime.IvarOffsetSymbol("Cache", c.name); got !=
			"OBJC_IVAR_$_Cache."+c.name {
			t.Errorf("offset symbol = %q", got)
		}
	}

	// The @synthesize named _name, so the property is backed by the ivar
	// the class already declared rather than by a new one.
	if n := len(cache.Ivars); n != 2 {
		t.Errorf("%d instance variables, want 2 — @synthesize reused _name", n)
	}
}

func log2(n int64) int64 {
	k := int64(0)
	for n > 1 {
		n >>= 1
		k++
	}
	return k
}

func TestSeamProperties(t *testing.T) {
	info := analyze(t)
	cache := findClass(info, "Cache")
	model := types.LP64()

	want := map[string]string{
		"name":  `T@"NSString",C,N,V_name`,
		"empty": "Ti,R,N,GisEmpty",
	}
	for _, p := range cache.Properties {
		desc := runtime.PropertyDesc{
			Type:      abi.EncodeExtended(p.Type, model),
			Readonly:  p.Has(types.PropReadonly),
			Copy:      p.Has(types.PropCopy),
			Retain:    p.Has(types.PropRetain) || p.Has(types.PropStrong),
			Weak:      p.Has(types.PropWeak),
			Nonatomic: p.Has(types.PropNonatomic),
			Dynamic:   p.Dynamic,
			Ivar:      p.Ivar,
		}
		// A custom getter or setter is only written when the program named
		// one; the defaults are implied by the property's name.
		if p.Has(types.PropGetter) {
			desc.Getter = p.Getter
		}
		if p.Has(types.PropSetter) {
			desc.Setter = p.Setter
		}
		if p.Has(types.PropReadonly) {
			// clang writes no backing variable for a readonly property the
			// class implements by hand.
			desc.Ivar = ""
		}
		got := runtime.PropertyAttributes(desc)
		if expect, ok := want[p.Name]; !ok {
			t.Errorf("unexpected property %q", p.Name)
		} else if got != expect {
			t.Errorf("%s: %q, want %q", p.Name, got, expect)
		}
	}
}

// The class flags and the symbols a class contributes, for a class with no
// ARC and nothing to clean up.
func TestSeamClass(t *testing.T) {
	info := analyze(t)
	cache := findClass(info, "Cache")

	if flags := runtime.ClassFlags(false, cache.Root, false, false, false); flags != 0 {
		t.Errorf("class flags = %#x, want 0", flags)
	}
	root := findClass(info, "NSObject")
	if !root.Root {
		t.Fatal("NSObject is a root class")
	}
	if flags := runtime.ClassFlags(false, root.Root, false, false, false); flags != runtime.RORoot {
		t.Errorf("root flags = %#x, want RORoot", flags)
	}
	if got := runtime.ClassSymbol(cache.Name); got != "OBJC_CLASS_$_Cache" {
		t.Errorf("class symbol = %q", got)
	}
	if cache.Super == nil || runtime.ClassSymbol(cache.Super.Name) != "OBJC_CLASS_$_NSObject" {
		t.Error("the superclass symbol is what the class object points at")
	}
}
