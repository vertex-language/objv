package types

import "testing"

// ---- the C model ----

func TestQualifiersDoNotNest(t *testing.T) {
	base := Typ(Int)
	c := Qualify(base, QConst)
	cv := Qualify(c, QVolatile)
	q, ok := cv.(*Qualified)
	if !ok {
		t.Fatalf("Qualify returned %T", cv)
	}
	if q.T != base {
		t.Error("qualifiers nested instead of merging")
	}
	if q.Q != QConst|QVolatile {
		t.Errorf("quals = %b, want const|volatile", q.Q)
	}
	if Qualify(base, 0) != Type(base) {
		t.Error("qualifying with nothing is the identity")
	}
	if Unqualify(cv) != Type(base) {
		t.Error("Unqualify strips the wrapper")
	}
}

// A lifetime and a nullability qualifier ride alongside the C ones, and
// replace rather than accumulate: a type has one owner.
func TestObjCQualifiers(t *testing.T) {
	obj := ID()
	strong := WithLifetime(obj, LifeStrong)
	weak := WithLifetime(strong, LifeWeak)
	if LifetimeOf(weak) != LifeWeak {
		t.Errorf("lifetime = %v, want __weak", LifetimeOf(weak))
	}
	both := WithNullability(Qualify(weak, QConst), NullNullable)
	if QualsOf(both) != QConst || LifetimeOf(both) != LifeWeak || NullabilityOf(both) != NullNullable {
		t.Errorf("the three qualifier sets do not coexist: %s", both)
	}
	if got, want := both.String(), "const __weak id _Nullable"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSizeofAndAlignof(t *testing.T) {
	m := LP64()
	for _, c := range []struct {
		t    Type
		size int64
	}{
		{Typ(Char), 1}, {Typ(Short), 2}, {Typ(Int), 4}, {Typ(Long), 8},
		{Typ(Double), 8}, {&Pointer{Elem: Typ(Int)}, 8},
		{ID(), 8}, {ClassObject(), 8},
		{&Block{Sig: &Func{Ret: Typ(Void), Proto: true}}, 8},
		{&Array{Elem: Typ(Int), Form: FixedArray, Len: 4}, 16},
	} {
		got, ok := m.Sizeof(c.t)
		if !ok || got != c.size {
			t.Errorf("Sizeof(%s) = %d, %v; want %d", c.t, got, ok, c.size)
		}
	}
	// An interface type has no size: a program holds a pointer to an
	// object, never an object.
	if _, ok := m.Sizeof(&Object{}); ok {
		t.Error("an interface type must have no size")
	}
	if _, ok := m.Sizeof(&Record{Name: "S"}); ok {
		t.Error("an incomplete record has no size")
	}
}

func TestRecordLayout(t *testing.T) {
	m := LP64()
	r := &Record{Name: "S", Complete: true, Fields: []Field{
		{Name: "a", Type: Typ(Char)},
		{Name: "b", Type: Typ(Int)},
		{Name: "c", Type: Typ(Char)},
	}}
	if size, _ := m.Sizeof(r); size != 12 {
		t.Errorf("size = %d, want 12", size)
	}
	if off, ok := m.Offsetof(r, "b"); !ok || off != 4 {
		t.Errorf("offsetof(b) = %d, want 4", off)
	}
	r.Packed = true
	if size, _ := m.Sizeof(r); size != 6 {
		t.Errorf("packed size = %d, want 6", size)
	}
}

// ---- Objective-C ----

// A class and a protocol have identity, and a subclass test is a walk.
func newHierarchy() (root, mid, leaf *Class, p *Protocol) {
	p = &Protocol{Name: "NSCopying", Complete: true}
	root = &Class{Name: "NSObject", Complete: true, Root: true}
	mid = &Class{Name: "NSString", Complete: true, Super: root, Protocols: []*Protocol{p}}
	leaf = &Class{Name: "NSMutableString", Complete: true, Super: mid}
	return
}

func TestClassRelations(t *testing.T) {
	root, mid, leaf, p := newHierarchy()
	if !leaf.IsSubclassOf(root) {
		t.Error("NSMutableString descends from NSObject")
	}
	if root.IsSubclassOf(leaf) {
		t.Error("the relation is not symmetric")
	}
	if !leaf.Conforms(p) {
		t.Error("conformance is inherited from the superclass")
	}
	if root.Conforms(p) {
		t.Error("NSObject does not adopt NSCopying here")
	}
	_ = mid
}

func TestObjectAssignment(t *testing.T) {
	root, mid, leaf, p := newHierarchy()
	other := &Class{Name: "NSNumber", Complete: true, Super: root}

	for _, c := range []struct {
		name     string
		dst, src Type
		want     AssignKind
	}{
		{"subclass to superclass", NewObject(mid), NewObject(leaf), AssignOK},
		{"superclass to subclass", NewObject(leaf), NewObject(mid), AssignObjCUnrelated},
		{"siblings", NewObject(mid), NewObject(other), AssignObjCUnrelated},
		{"to id", ID(), NewObject(mid), AssignOK},
		{"from id", NewObject(mid), ID(), AssignOK},
		{"to a conforming qualified id", NewObject(nil, p), NewObject(mid), AssignOK},
		{"to a qualified id it does not conform to", NewObject(nil, p), NewObject(other), AssignObjCProtocol},
		{"Class to id", ID(), ClassObject(), AssignOK},
		{"Class to an instance pointer", NewObject(mid), ClassObject(), AssignObjCUnrelated},
		{"an int", NewObject(mid), Typ(Int), AssignIntPointer},
	} {
		if got := Assignable(c.dst, c.src, false); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}

	// A null pointer constant assigns to any object pointer.
	if got := Assignable(NewObject(mid), Typ(Int), true); got != AssignOK {
		t.Errorf("nil to an object pointer: %v", got)
	}

	// __kindof admits the downcast the qualifier exists for.
	kindOf := Qualify(NewObject(mid), QKindOf)
	if got := Assignable(NewObject(leaf), kindOf, false); got != AssignOK {
		t.Errorf("__kindof NSString * to NSMutableString *: %v", got)
	}
}

func TestGenericAssignment(t *testing.T) {
	root, mid, leaf, _ := newHierarchy()
	elem := &TypeParam{Name: "ObjectType", Bound: ID()}
	array := &Class{Name: "NSArray", Complete: true, Super: root,
		TypeParams: []*TypeParam{elem}}
	spec := func(arg Type) Type {
		return &Pointer{Elem: &Object{Base: array, Args: []Type{arg}}}
	}

	if got := Assignable(spec(NewObject(mid)), spec(NewObject(mid)), false); got != AssignOK {
		t.Errorf("the same specialization: %v", got)
	}
	// Invariant by default: an array of subclasses is not an array of the
	// superclass, because the array is writable.
	if got := Assignable(spec(NewObject(mid)), spec(NewObject(leaf)), false); got != AssignObjCTypeArgs {
		t.Errorf("invariance: %v, want a type-argument mismatch", got)
	}
	// __covariant is how a class opts out of that.
	elem.Variance = Covariant
	if got := Assignable(spec(NewObject(mid)), spec(NewObject(leaf)), false); got != AssignOK {
		t.Errorf("covariance: %v", got)
	}
	// An unspecialized type converts either way: the runtime erases the
	// arguments anyway.
	if got := Assignable(NewObject(array), spec(NewObject(mid)), false); got != AssignOK {
		t.Errorf("to the unspecialized type: %v", got)
	}
}

func TestBlockAssignment(t *testing.T) {
	void := func(params ...Type) *Block {
		f := &Func{Ret: Typ(Void), Proto: true}
		for _, p := range params {
			f.Params = append(f.Params, Param{Type: p})
		}
		return &Block{Sig: f}
	}
	if got := Assignable(void(), void(), false); got != AssignOK {
		t.Errorf("identical signatures: %v", got)
	}
	if got := Assignable(void(Typ(Int)), void(), false); got != AssignPointerMismatch {
		t.Errorf("different signatures: %v", got)
	}
	// A block is an object.
	if got := Assignable(ID(), void(), false); got != AssignOK {
		t.Errorf("a block assigns to id: %v", got)
	}
	if !IsObjCObject(void()) {
		t.Error("a block is something the runtime retains")
	}
	if !IsScalar(void()) {
		t.Error("a block pointer is scalar: `if (block)` asks about it")
	}
}

// Whether a conversion crosses the boundary ARC manages is a fact about the
// types; whether crossing it needs a keyword depends on ARC being on, which
// this package does not know.
func TestBridge(t *testing.T) {
	voidPtr := &Pointer{Elem: Typ(Void)}
	if Bridge(voidPtr, ID()) != BridgeNeeded {
		t.Error("id to void * crosses the boundary")
	}
	if Bridge(ID(), voidPtr) != BridgeNeeded {
		t.Error("void * to id crosses it too")
	}
	if Bridge(ID(), NewObject(&Class{Name: "NSString"})) != BridgeNone {
		t.Error("two object pointers do not")
	}
	if Bridge(voidPtr, &Pointer{Elem: Typ(Int)}) != BridgeNone {
		t.Error("two ordinary pointers do not")
	}
}

func TestMethodString(t *testing.T) {
	m := &Method{
		Sel: "setObject:forKey:", Ret: Typ(Void),
		Params: []Param{{Name: "object", Type: ID()}, {Name: "key", Type: ID()}},
	}
	if got, want := m.String(), "- (void)setObject:(id)object forKey:(id)key"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	u := &Method{Sel: "init", Class: false, Ret: Instancetype()}
	if got, want := u.String(), "- (instancetype)init"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSelectorAssembly(t *testing.T) {
	if got := Selector([]string{"setObject", "forKey"}, true); got != "setObject:forKey:" {
		t.Errorf("Selector = %q", got)
	}
	if got := Selector([]string{"init"}, false); got != "init" {
		t.Errorf("Selector = %q", got)
	}
	// A nameless keyword piece contributes its colon and nothing else.
	if got := Selector([]string{"a", ""}, true); got != "a::" {
		t.Errorf("Selector = %q", got)
	}
}

func TestLookup(t *testing.T) {
	_, mid, leaf, p := newHierarchy()
	p.Methods = []*Method{{Sel: "copyWithZone:", Ret: ID(), Owner: "NSCopying"}}
	mid.Methods = []*Method{
		{Sel: "length", Ret: Typ(Int), Owner: "NSString"},
		{Sel: "string", Class: true, Ret: Instancetype(), Owner: "NSString"},
	}
	if m := leaf.Lookup("length", false); m == nil || m.Owner != "NSString" {
		t.Error("a method is found on the superclass")
	}
	if m := leaf.Lookup("length", true); m != nil {
		t.Error("a class method and an instance method are different methods")
	}
	if m := leaf.Lookup("string", true); m == nil {
		t.Error("the class method is found")
	}
	if m := leaf.Lookup("copyWithZone:", false); m == nil {
		t.Error("a method declared by an adopted protocol is declared for the class")
	}
	if m := leaf.Lookup("nope", false); m != nil {
		t.Error("a method nobody declares is not found")
	}
}

func TestPropertyLookup(t *testing.T) {
	_, mid, leaf, p := newHierarchy()
	p.Properties = []*Property{{Name: "copyable", Type: ID(), Owner: "NSCopying"}}
	mid.Properties = []*Property{{Name: "length", Type: Typ(Int),
		Getter: "length", Setter: "setLength:", Owner: "NSString"}}
	if pr := leaf.FindProperty("length"); pr == nil || pr.Getter != "length" {
		t.Error("a property is found on the superclass, with its accessors")
	}
	if pr := leaf.FindProperty("copyable"); pr == nil {
		t.Error("a property declared by an adopted protocol is found")
	}
}

func TestObjectPrinting(t *testing.T) {
	root, mid, _, p := newHierarchy()
	array := &Class{Name: "NSArray", Super: root}
	for _, c := range []struct {
		t    Type
		want string
	}{
		// `id` is a pointer whose spelling has no star; `NSString *` has one.
		{ID(), "id"},
		{ClassObject(), "Class"},
		{NewObject(mid), "NSString*"},
		{NewObject(nil, p), "id<NSCopying>"},
		{&Pointer{Elem: &Object{Base: array, Args: []Type{NewObject(mid)}}}, "NSArray<NSString*>*"},
		{Qualify(NewObject(mid), QKindOf), "__kindof NSString*"},
		{&Block{Sig: &Func{Ret: Typ(Void), Params: []Param{{Type: ID()}}, Proto: true}}, "void(^)(id)"},
	} {
		if got := c.t.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestCompatible(t *testing.T) {
	_, mid, _, p := newHierarchy()
	if !Compatible(NewObject(mid), NewObject(mid)) {
		t.Error("the same class is the same type")
	}
	if Compatible(NewObject(mid), NewObject(nil)) {
		t.Error("NSString * and id are not the same type")
	}
	if Compatible(NewObject(nil, p), NewObject(nil)) {
		t.Error("id<NSCopying> and id are not the same type")
	}
	if !Compatible(Typ(Int), Typ(Int)) {
		t.Error("int is int")
	}
}

func TestEnumFixedUnderlyingType(t *testing.T) {
	// NS_ENUM's enum: the constants have the enumeration's type, which is
	// what a switch over one and an NSInteger parameter both depend on.
	e := &Enum{Name: "NSComparisonResult", Complete: true, Fixed: true, Under: Long}
	if e.ConstType() != Type(e) {
		t.Error("with a fixed underlying type, a constant has the enumeration's type")
	}
	m := LP64()
	if size, ok := m.Sizeof(e); !ok || size != 8 {
		t.Errorf("size = %d, want the underlying type's 8", size)
	}
	plain := &Enum{Name: "E", Complete: true}
	if plain.ConstType() != Type(Typ(Int)) {
		t.Error("without one, a constant is an int")
	}
}

// FieldOffsets answers by position where Offsetof answers by name. An
// initializer needs the former: it walks the members in order, and an
// anonymous member has no name to ask about.
func TestFieldOffsets(t *testing.T) {
	m := LP64()
	inner := &Record{Name: "inner", Complete: true, Fields: []Field{
		{Name: "x", Type: Typ(Int)},
	}}
	r := &Record{Name: "s", Complete: true, Fields: []Field{
		{Name: "a", Type: Typ(Char)},
		{Name: "", Type: inner}, // anonymous, so Offsetof cannot name it
		{Name: "c", Type: Typ(Double)},
	}}
	offs, ok := m.FieldOffsets(r)
	if !ok {
		t.Fatal("FieldOffsets refused a complete record")
	}
	if want := []int64{0, 4, 8}; len(offs) != 3 ||
		offs[0] != want[0] || offs[1] != want[1] || offs[2] != want[2] {
		t.Errorf("offsets = %v, want %v", offs, want)
	}

	// An incomplete record has no layout to report.
	if _, ok := m.FieldOffsets(&Record{Name: "opaque"}); ok {
		t.Error("FieldOffsets answered for an incomplete record")
	}
	if _, ok := m.FieldOffsets(nil); ok {
		t.Error("FieldOffsets answered for a nil record")
	}

	// A union's members all begin at zero.
	u := &Record{Name: "u", Union: true, Complete: true, Fields: []Field{
		{Name: "i", Type: Typ(Int)},
		{Name: "d", Type: Typ(Double)},
	}}
	offs, ok = m.FieldOffsets(u)
	if !ok || offs[0] != 0 || offs[1] != 0 {
		t.Errorf("union offsets = %v, want [0 0]", offs)
	}
}

// §7.16's list is a target fact and not a language one, and the two answers
// are not variations on each other: a pointer where every variadic argument
// takes one stack slot, and four fields where the walk has two regions to
// cross.
func TestVaListSizeIsTheTargets(t *testing.T) {
	if got := LP64().VaListSize; got != 0 {
		t.Errorf("the bare model states a size (%d); the target is what states one", got)
	}
}
