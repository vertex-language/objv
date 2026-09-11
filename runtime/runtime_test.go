package runtime_test

import (
	"testing"

	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Every expected string in this file came out of clang. The ABI is a
// contract with a runtime nobody here controls, so the only test worth
// having is one whose answers were taken from the compiler that already
// speaks it:
//
//	xcrun clang -S -fobjc-arc t.m -o -   for the metadata
//	printf("%s", @encode(T))             for the encodings

var abi = runtime.Darwin64()

func m() types.Model { return types.LP64() }

// ---- encodings ----

func TestEncodeBasics(t *testing.T) {
	for _, c := range []struct {
		t    types.Type
		want string
	}{
		{types.Typ(types.Void), "v"},
		{types.Typ(types.Bool), "B"},
		{types.Typ(types.Char), "c"},
		{types.Typ(types.SChar), "c"},
		{types.Typ(types.UChar), "C"},
		{types.Typ(types.Short), "s"},
		{types.Typ(types.UShort), "S"},
		{types.Typ(types.Int), "i"},
		{types.Typ(types.UInt), "I"},
		{types.Typ(types.Long), "q"}, // 'q' on LP64, as clang has it
		{types.Typ(types.ULong), "Q"},
		{types.Typ(types.LongLong), "q"},
		{types.Typ(types.ULongLong), "Q"},
		{types.Typ(types.Float), "f"},
		{types.Typ(types.Double), "d"},
		{types.Typ(types.LongDouble), "D"},
	} {
		if got := abi.Encode(c.t, m()); got != c.want {
			t.Errorf("@encode(%s) = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestEncodePointers(t *testing.T) {
	ptr := func(t types.Type) types.Type { return &types.Pointer{Elem: t} }
	konst := func(t types.Type) types.Type { return types.Qualify(t, types.QConst) }
	sel := types.NewSelector()

	for _, c := range []struct {
		name string
		t    types.Type
		want string
	}{
		{"void *", ptr(types.Typ(types.Void)), "^v"},
		{"int *", ptr(types.Typ(types.Int)), "^i"},
		{"int **", ptr(ptr(types.Typ(types.Int))), "^^i"},

		// A pointer to a character is the one type the runtime treats as a
		// string, and it is not '^c'.
		{"char *", ptr(types.Typ(types.Char)), "*"},
		{"const char *", ptr(konst(types.Typ(types.Char))), "r*"},
		{"char **", ptr(ptr(types.Typ(types.Char))), "^*"},

		// const reaches the encoding as an 'r' in front of the pointer.
		{"const int *", ptr(konst(types.Typ(types.Int))), "r^i"},
		// A top-level const is dropped.
		{"int *const", konst(ptr(types.Typ(types.Int))), "^i"},
		{"const int", konst(types.Typ(types.Int)), "i"},
		// volatile is not in the alphabet at all.
		{"volatile int *", ptr(types.Qualify(types.Typ(types.Int), types.QVolatile)), "^i"},

		{"id", types.ID(), "@"},
		{"Class", types.ClassObject(), "#"},
		{"SEL", sel, ":"},
		{"SEL *", ptr(sel), "^:"},
		{"Class *", ptr(types.ClassObject()), "^#"},
		{"NSString *", types.NewObject(&types.Class{Name: "NSString"}), "@"},

		// A block is '@?' — an object whose class the encoding does not
		// name — and a function pointer is '^?'.
		{"block", &types.Block{Sig: &types.Func{Ret: types.Typ(types.Void), Proto: true}}, "@?"},
		{"fn *", ptr(&types.Func{Ret: types.Typ(types.Int), Proto: true}), "^?"},
	} {
		if got := abi.Encode(c.t, m()); got != c.want {
			t.Errorf("@encode(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func point() *types.Record {
	return &types.Record{Name: "Point", Complete: true, Fields: []types.Field{
		{Name: "x", Type: types.Typ(types.Int)},
		{Name: "y", Type: types.Typ(types.Float)},
	}}
}

func TestEncodeAggregates(t *testing.T) {
	p := point()
	u := &types.Record{Name: "U", Union: true, Complete: true, Fields: []types.Field{
		{Name: "i", Type: types.Typ(types.Int)},
		{Name: "c", Type: types.Typ(types.Char)},
	}}
	anon := &types.Record{Complete: true, Fields: []types.Field{
		{Name: "a", Type: &types.Array{Elem: types.Typ(types.Int), Form: types.FixedArray, Len: 3}},
	}}
	bits := &types.Record{Name: "Bits", Complete: true, Fields: []types.Field{
		{Name: "a", Type: types.Typ(types.UInt), BitField: true, Width: 3},
		{Name: "b", Type: types.Typ(types.UInt), BitField: true, Width: 5},
	}}
	empty := &types.Record{Name: "Empty", Complete: true}

	for _, c := range []struct {
		name string
		t    types.Type
		want string
	}{
		{"struct Point", p, "{Point=if}"},
		{"union U", u, "(U=ic)"},
		{"anonymous", anon, "{?=[3i]}"},
		{"bit-fields", bits, "{Bits=b3b5}"},
		{"empty", empty, "{Empty=}"},
		{"int[4]", &types.Array{Elem: types.Typ(types.Int), Form: types.FixedArray, Len: 4}, "[4i]"},
		{"int[2][3]", &types.Array{Form: types.FixedArray, Len: 2,
			Elem: &types.Array{Elem: types.Typ(types.Int), Form: types.FixedArray, Len: 3}}, "[2[3i]]"},
		{"struct Point *", &types.Pointer{Elem: p}, "^{Point=if}"},
		{"struct Point[2]", &types.Array{Elem: p, Form: types.FixedArray, Len: 2}, "[2{Point=if}]"},
	} {
		if got := abi.Encode(c.t, m()); got != c.want {
			t.Errorf("@encode(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

// A struct that contains a pointer to itself has to stop somewhere, and the
// runtime's answer is that a pointer to a struct already being written is
// the tag alone.
func TestEncodeRecursiveRecord(t *testing.T) {
	node := &types.Record{Name: "Node", Complete: true}
	node.Fields = []types.Field{
		{Name: "v", Type: types.Typ(types.Int)},
		{Name: "next", Type: &types.Pointer{Elem: node}},
	}
	if got, want := abi.Encode(node, m()), "{Node=i^{Node}}"; got != want {
		t.Errorf("Encode = %q, want %q", got, want)
	}
	outer := &types.Record{Name: "Outer", Complete: true, Fields: []types.Field{
		{Name: "n", Type: node},
		{Name: "m", Type: types.Typ(types.Int)},
	}}
	// A nested struct by value still expands: only the cycle stops.
	if got, want := abi.Encode(outer, m()), "{Outer={Node=i^{Node}}i}"; got != want {
		t.Errorf("Encode = %q, want %q", got, want)
	}
}

// The extended encoding keeps the class, which is what an instance variable
// and a property carry.
func TestEncodeExtended(t *testing.T) {
	ns := &types.Class{Name: "NSObject"}
	copying := &types.Protocol{Name: "Copy"}
	for _, c := range []struct {
		name string
		t    types.Type
		want string
	}{
		{"NSObject *", types.NewObject(ns), `@"NSObject"`},
		{"id<Copy>", types.NewObject(nil, copying), `@"<Copy>"`},
		{"id", types.ID(), "@"},
		{"int", types.Typ(types.Int), "i"},
	} {
		if got := abi.EncodeExtended(c.t, m()); got != c.want {
			t.Errorf("extended(%s) = %q, want %q", c.name, got, c.want)
		}
	}
	// The plain form says only that it is an object.
	if got := abi.Encode(types.NewObject(ns), m()); got != "@" {
		t.Errorf("@encode of an object is %q, want %q", got, "@")
	}
}

// ---- method type strings ----

func TestMethodTypes(t *testing.T) {
	id := types.ID()
	big := &types.Record{Name: "Big", Complete: true, Fields: []types.Field{
		{Name: "a", Type: types.Typ(types.Double)},
		{Name: "b", Type: types.Typ(types.Double)},
		{Name: "c", Type: types.Typ(types.Double)},
		{Name: "d", Type: types.Typ(types.Double)},
		{Name: "e", Type: types.Typ(types.Double)},
	}}

	for _, c := range []struct {
		name   string
		ret    types.Type
		params []types.Param
		want   string
	}{
		// - (id)shared            self@0, _cmd@8, frame 16
		{"-shared", id, nil, "@16@0:8"},
		{"-sends", types.Typ(types.Void), nil, "v16@0:8"},
		// - (int)feed:(int)n      the int lands at 16 and the frame is 20:
		// the frame is the sum of the arguments, not rounded up at the end.
		{"-feed:", types.Typ(types.Int),
			[]types.Param{{Type: types.Typ(types.Int)}}, "i20@0:8i16"},
		{"-setName:", types.Typ(types.Void),
			[]types.Param{{Type: id}}, "v24@0:8@16"},
		{"-objectForKey:", id, []types.Param{{Type: id}}, "@24@0:8@16"},
		// A struct returned indirectly is not in the frame.
		{"-big", big, nil, "{Big=ddddd}16@0:8"},
		// A struct passed by value is: 16 + 40.
		{"-take:", types.Typ(types.Void),
			[]types.Param{{Type: big}}, "v56@0:8{Big=ddddd}16"},
	} {
		if got := abi.MethodTypes(c.ret, c.params, m()); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
}

// ---- symbols ----

func TestSymbols(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{runtime.ClassSymbol("Cache"), "OBJC_CLASS_$_Cache"},
		{runtime.MetaclassSymbol("Cache"), "OBJC_METACLASS_$_Cache"},
		{runtime.ClassROSymbol("Cache"), "_OBJC_CLASS_RO_$_Cache"},
		{runtime.MetaclassROSymbol("Cache"), "_OBJC_METACLASS_RO_$_Cache"},
		{runtime.IvarOffsetSymbol("Cache", "_count"), "OBJC_IVAR_$_Cache._count"},
		{runtime.InstanceMethodsSymbol("Cache"), "_OBJC_$_INSTANCE_METHODS_Cache"},
		{runtime.ClassMethodsSymbol("Cache"), "_OBJC_$_CLASS_METHODS_Cache"},
		{runtime.IvarsSymbol("Cache"), "_OBJC_$_INSTANCE_VARIABLES_Cache"},
		{runtime.PropertiesSymbol("Cache"), "_OBJC_$_PROP_LIST_Cache"},
		{runtime.ClassProtocolsSymbol("Cache"), "_OBJC_CLASS_PROTOCOLS_$_Cache"},
		{runtime.ProtocolSymbol("Feeder"), "_OBJC_PROTOCOL_$_Feeder"},
		{runtime.ProtocolLabelSymbol("Feeder"), "_OBJC_LABEL_PROTOCOL_$_Feeder"},
		{runtime.ProtocolInstanceMethodsSymbol("Feeder"), "_OBJC_$_PROTOCOL_INSTANCE_METHODS_Feeder"},
		{runtime.ProtocolOptionalInstanceMethodsSymbol("Feeder"), "_OBJC_$_PROTOCOL_INSTANCE_METHODS_OPT_Feeder"},
		{runtime.ProtocolMethodTypesSymbol("Feeder"), "_OBJC_$_PROTOCOL_METHOD_TYPES_Feeder"},
		{runtime.CategorySymbol("Cache", "Extra"), "_OBJC_$_CATEGORY_Cache_$_Extra"},
		{runtime.CategoryInstanceMethodsSymbol("Cache", "Extra"),
			"_OBJC_$_CATEGORY_INSTANCE_METHODS_Cache_$_Extra"},

		// A method's symbol is spelled the way the language spells the
		// method, which is why a crash report is readable.
		{runtime.MethodName("NSString", "", "length", false), "-[NSString length]"},
		{runtime.MethodName("Cache", "", "shared", true), "+[Cache shared]"},
		{runtime.MethodName("Cache", "Extra", "extra", false), "-[Cache(Extra) extra]"},
		{runtime.MethodName("Cache", "", "setObject:forKey:", false),
			"-[Cache setObject:forKey:]"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

// ---- sections ----

func TestSections(t *testing.T) {
	for _, c := range []struct {
		s    runtime.Section
		want string
	}{
		{runtime.SecClassList, "__DATA,__objc_classlist,regular,no_dead_strip"},
		{runtime.SecCategoryList, "__DATA,__objc_catlist,regular,no_dead_strip"},
		{runtime.SecProtocolList, "__DATA,__objc_protolist,coalesced,no_dead_strip"},
		{runtime.SecImageInfo, "__DATA,__objc_imageinfo,regular,no_dead_strip"},
		{runtime.SecClassData, "__DATA,__objc_data"},
		{runtime.SecConst, "__DATA,__objc_const"},
		{runtime.SecIvarOffsets, "__DATA,__objc_ivar"},
		{runtime.SecSelectorRefs, "__DATA,__objc_selrefs,literal_pointers,no_dead_strip"},
		{runtime.SecClassRefs, "__DATA,__objc_classrefs,regular,no_dead_strip"},
		{runtime.SecSuperRefs, "__DATA,__objc_superrefs,regular,no_dead_strip"},
		{runtime.SecClassNames, "__TEXT,__objc_classname,cstring_literals"},
		{runtime.SecMethodNames, "__TEXT,__objc_methname,cstring_literals"},
		{runtime.SecMethodTypes, "__TEXT,__objc_methtype,cstring_literals"},
	} {
		if got := abi.Name(c.s); got != c.want {
			t.Errorf("section = %q, want %q", got, c.want)
		}
	}
	// ELF has no segments and no attributes.
	elf := runtime.ABI{Container: runtime.ELF, PtrBytes: 8}
	if got := elf.Name(runtime.SecClassList); got != "__objc_classlist" {
		t.Errorf("ELF section = %q", got)
	}
}

// ---- layouts ----

// The sizes are the ones the metadata itself carries: a protocol writes 96
// into its own size field and a category writes 64, and both are read back
// by a runtime that may be older than the compiler.
func TestLayoutSizes(t *testing.T) {
	for _, c := range []struct {
		name   string
		fields []runtime.Field
		want   int64
	}{
		{"class_t", runtime.Class, 40},
		{"class_ro_t", runtime.ClassRO, 72},
		{"method_t", runtime.Method, 24},
		{"ivar_t", runtime.Ivar, 32},
		{"property_t", runtime.Property, 16},
		{"protocol_t", runtime.Protocol, 96},
		{"category_t", runtime.Category, 64},
		{"image_info", runtime.ImageInfo, 8},
	} {
		if got := abi.SizeOf(c.fields); got != c.want {
			t.Errorf("sizeof(%s) = %d, want %d", c.name, got, c.want)
		}
	}
	// The entsize a list header carries is the entry's size.
	if got := abi.EntSize(runtime.Method); got != 24 {
		t.Errorf("method list entsize = %d, want 24", got)
	}
	if got := abi.EntSize(runtime.Ivar); got != 32 {
		t.Errorf("ivar list entsize = %d, want 32", got)
	}
	if got := abi.EntSize(runtime.Property); got != 16 {
		t.Errorf("property list entsize = %d, want 16", got)
	}
}

func TestLayoutOffsets(t *testing.T) {
	for _, c := range []struct {
		field string
		want  int64
	}{
		{"flags", 0}, {"instanceStart", 4}, {"instanceSize", 8},
		{"ivarLayout", 16}, {"name", 24}, {"baseMethodList", 32},
		{"baseProtocols", 40}, {"ivars", 48}, {"weakIvarLayout", 56},
		{"baseProperties", 64},
	} {
		got, ok := abi.OffsetOf(runtime.ClassRO, c.field)
		if !ok || got != c.want {
			t.Errorf("class_ro_t.%s at %d (%v), want %d", c.field, got, ok, c.want)
		}
	}
	if _, ok := abi.OffsetOf(runtime.ClassRO, "nothing"); ok {
		t.Error("a field nobody declared has no offset")
	}
	// The four bytes of padding are why every pointer field after them is
	// where it is.
	if off, _ := abi.OffsetOf(runtime.ClassRO, "ivarLayout"); off != 16 {
		t.Errorf("the reserved word was dropped: ivarLayout at %d", off)
	}
}

// ---- flags ----

func TestClassFlags(t *testing.T) {
	// What clang wrote for an ARC class with a .cxx_destruct, and for its
	// metaclass: 0x184 and 0x185.
	got := runtime.ClassFlags(false, false, true, true, false)
	if got != 0x184 {
		t.Errorf("class flags = %#x, want %#x", got, 0x184)
	}
	meta := runtime.ClassFlags(true, false, true, true, false)
	if meta != 0x185 {
		t.Errorf("metaclass flags = %#x, want %#x", meta, 0x185)
	}
	// A root class without ARC and without ivars to clean up.
	plain := runtime.ClassFlags(false, true, false, false, false)
	if plain != runtime.RORoot {
		t.Errorf("root flags = %#x, want %#x", plain, runtime.RORoot)
	}
	if runtime.ImageInfoFlags() != 64 {
		t.Errorf("image info flags = %d, want 64", runtime.ImageInfoFlags())
	}
}

func TestPropertyAttributes(t *testing.T) {
	for _, c := range []struct {
		name string
		p    runtime.PropertyDesc
		want string
	}{
		{"nonatomic copy object",
			runtime.PropertyDesc{Type: `@"NSObject"`, Copy: true, Nonatomic: true, Ivar: "_c"},
			`T@"NSObject",C,N,V_c`},
		{"atomic strong",
			runtime.PropertyDesc{Type: `@"NSObject"`, Retain: true, Ivar: "_s"},
			`T@"NSObject",&,V_s`},
		{"weak qualified id",
			runtime.PropertyDesc{Type: `@"<Copy>"`, Weak: true, Nonatomic: true, Ivar: "_w"},
			`T@"<Copy>",W,N,V_w`},
		{"plain int",
			runtime.PropertyDesc{Type: "i", Nonatomic: true, Ivar: "_i"},
			"Ti,N,V_i"},
		{"unsafe_unretained says nothing",
			runtime.PropertyDesc{Type: `@"NSObject"`, Nonatomic: true, Ivar: "_u"},
			`T@"NSObject",N,V_u`},
		{"readonly with a getter",
			runtime.PropertyDesc{Type: "i", Readonly: true, Getter: "isOn"},
			"Ti,R,GisOn"},
		{"custom setter",
			runtime.PropertyDesc{Type: "i", Nonatomic: true, Setter: "put:", Ivar: "_put"},
			"Ti,N,Sput:,V_put"},
		{"dynamic",
			runtime.PropertyDesc{Type: "i", Dynamic: true, Nonatomic: true},
			"Ti,D,N"},
		{"block",
			runtime.PropertyDesc{Type: "@?", Copy: true, Nonatomic: true, Ivar: "_blk"},
			"T@?,C,N,V_blk"},
		{"anonymous struct",
			runtime.PropertyDesc{Type: "{?=i}", Nonatomic: true, Ivar: "_anon"},
			"T{?=i},N,V_anon"},
	} {
		if got := runtime.PropertyAttributes(c.p); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// ---- dispatch ----

func TestSendVariants(t *testing.T) {
	big := &types.Record{Name: "Big", Complete: true, Fields: []types.Field{
		{Type: types.Typ(types.Double)}, {Type: types.Typ(types.Double)},
		{Type: types.Typ(types.Double)}, {Type: types.Typ(types.Double)},
	}}
	small := &types.Record{Name: "Small", Complete: true, Fields: []types.Field{
		{Type: types.Typ(types.Int)}, {Type: types.Typ(types.Int)},
	}}

	for _, c := range []struct {
		name  string
		arch  runtime.Arch
		ret   types.Type
		super bool
		want  string
	}{
		{"arm64 id", runtime.ARM64, types.ID(), false, runtime.MsgSend},
		{"arm64 super", runtime.ARM64, types.ID(), true, runtime.MsgSendSuper2},
		// arm64 has no stret: the indirect result register is separate, so
		// the ordinary trampoline forwards it.
		{"arm64 big struct", runtime.ARM64, big, false, runtime.MsgSend},
		{"arm64 double", runtime.ARM64, types.Typ(types.Double), false, runtime.MsgSend},

		{"amd64 id", runtime.AMD64, types.ID(), false, runtime.MsgSend},
		{"amd64 small struct", runtime.AMD64, small, false, runtime.MsgSend},
		{"amd64 big struct", runtime.AMD64, big, false, runtime.MsgSendStret},
		{"amd64 big struct super", runtime.AMD64, big, true, runtime.MsgSendSuper2Stret},
		{"amd64 double", runtime.AMD64, types.Typ(types.Double), false, runtime.MsgSend},
		{"amd64 long double", runtime.AMD64, types.Typ(types.LongDouble), false, runtime.MsgSendFpret},

		// i386 returns every struct indirectly and every float on the x87
		// stack.
		{"i386 small struct", runtime.I386, small, false, runtime.MsgSendStret},
		{"i386 double", runtime.I386, types.Typ(types.Double), false, runtime.MsgSendFpret},
		{"i386 int", runtime.I386, types.Typ(types.Int), false, runtime.MsgSend},
	} {
		if got := abi.Send(c.arch, c.ret, m(), c.super); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// The constant-string ABI, read off `clang -S` for a file containing
// nothing but @"hi" and @"héllo".
func TestConstantString(t *testing.T) {
	abi := runtime.Darwin64()

	// clang emits an isa, a flags word, four bytes of padding, a data
	// pointer and a length: four quads' worth on a 64-bit target.
	if got := abi.SizeOf(runtime.ConstantString); got != 32 {
		t.Errorf("sizeof CFString = %d, want 32", got)
	}
	for _, c := range []struct {
		field string
		off   int64
	}{{"isa", 0}, {"flags", 8}, {"data", 16}, {"length", 24}} {
		got, ok := abi.OffsetOf(runtime.ConstantString, c.field)
		if !ok || got != c.off {
			t.Errorf("offsetof(%s) = %d, %v; want %d", c.field, got, ok, c.off)
		}
	}

	// 0x7C8 and 0x7D0 are CoreFoundation's own, and are the only two
	// clang emits: an 8-bit string and a UTF-16 one.
	if runtime.CFStringASCII != 0x7C8 || runtime.CFStringUTF16 != 0x7D0 {
		t.Errorf("flags %#x/%#x, want 0x7c8/0x7d0",
			runtime.CFStringASCII, runtime.CFStringUTF16)
	}

	// The isa is CoreFoundation's on Darwin and libobjc2's elsewhere. The
	// name carries no platform underscore; the object writer adds it.
	if got := abi.ConstantStringClass(); got != "__CFConstantStringClassReference" {
		t.Errorf("Darwin isa = %q", got)
	}
	gnu := runtime.ABI{Kind: runtime.GNUstep, Container: runtime.ELF, PtrBytes: 8}
	if got := gnu.ConstantStringClass(); got != "_NSConstantStringClassReference" {
		t.Errorf("GNUstep isa = %q", got)
	}
}

// The sections a constant string lives in. The two character sections
// differ only in the width of a code unit, and neither is writable.
func TestConstantStringSections(t *testing.T) {
	abi := runtime.Darwin64()
	for _, c := range []struct{ sec, want string }{
		{"cfstring", "__DATA,__cfstring"},
		{"cstring", "__TEXT,__cstring,cstring_literals"},
		{"ustring", "__TEXT,__ustring"},
	} {
		var s runtime.Section
		switch c.sec {
		case "cfstring":
			s = runtime.SecCFString
		case "cstring":
			s = runtime.SecCString
		case "ustring":
			s = runtime.SecUString
		}
		if got := abi.Name(s); got != c.want {
			t.Errorf("%s section = %q, want %q", c.sec, got, c.want)
		}
	}
}

// NSFastEnumerationState is three words the loop reads and five it does not.
func TestFastEnumerationState(t *testing.T) {
	abi := runtime.Darwin64()
	if got := abi.SizeOf(runtime.FastEnumerationState); got != 64 {
		t.Errorf("sizeof NSFastEnumerationState = %d, want 64", got)
	}
	for _, c := range []struct {
		field string
		off   int64
	}{{"state", 0}, {"itemsPtr", 8}, {"mutationsPtr", 16}} {
		got, ok := abi.OffsetOf(runtime.FastEnumerationState, c.field)
		if !ok || got != c.off {
			t.Errorf("offsetof(%s) = %d, %v; want %d", c.field, got, ok, c.off)
		}
	}
}

// A block's invoke function has one hidden argument where a method has two,
// and it is the block itself. The string is what clang writes into the
// descriptor for `int (^)(int)`.
func TestBlockTypes(t *testing.T) {
	a := runtime.Darwin64()
	got := a.BlockTypes(types.Typ(types.Int), []types.Param{{Type: types.Typ(types.Int)}}, types.LP64())
	if want := "i12@?0i8"; got != want {
		t.Errorf("BlockTypes = %q, want %q", got, want)
	}
	got = a.BlockTypes(types.Typ(types.Void), nil, types.LP64())
	if want := "v8@?0"; got != want {
		t.Errorf("BlockTypes of a void block = %q, want %q", got, want)
	}
}

// The layouts, against Block-ABI-Apple.txt and against what clang emits: a
// literal with no captures is 32 bytes, its descriptor 32, and a descriptor
// carrying the two helpers 48.
func TestBlockLayouts(t *testing.T) {
	a := runtime.Darwin64()
	for _, c := range []struct {
		name   string
		fields []runtime.Field
		want   int64
	}{
		{"block_literal", runtime.BlockLiteral, 32},
		{"block_descriptor", runtime.BlockDescriptor, 32},
		{"block_descriptor_2", runtime.BlockDescriptorWithHelpers, 48},
	} {
		if got := a.SizeOf(c.fields); got != c.want {
			t.Errorf("sizeof %s = %d, want %d", c.name, got, c.want)
		}
	}
	// invoke is at 16 and the descriptor at 24: a call site loads invoke
	// from a fixed offset and nothing else about the block matters to it.
	if off, ok := a.OffsetOf(runtime.BlockLiteral, "invoke"); !ok || off != 16 {
		t.Errorf("invoke at %d, want 16", off)
	}
	if off, ok := a.OffsetOf(runtime.BlockLiteral, "descriptor"); !ok || off != 24 {
		t.Errorf("descriptor at %d, want 24", off)
	}
}

// The flags clang writes: 0x40000000 on a stack block and 0x50000000 on a
// global one, both read out of an -S listing.
func TestBlockFlags(t *testing.T) {
	if got := runtime.BlockHasSignature; got != 0x40000000 {
		t.Errorf("BlockHasSignature = %#x", uint32(got))
	}
	if got := runtime.BlockIsGlobal | runtime.BlockHasSignature; got != 0x50000000 {
		t.Errorf("global block flags = %#x", uint32(got))
	}
	if got := runtime.BlockHasCopyDispose; got != 0x2000000 {
		t.Errorf("BlockHasCopyDispose = %#x", uint32(got))
	}
}
