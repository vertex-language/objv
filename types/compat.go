package types

// Type classification, the conversions C11 §6.3 defines over it, §6.2.7's
// compatibility, and the Objective-C rules that sit on top of both.
//
// These live here rather than in one of the packages that needs them because
// both do: the analyzer decides whether an assignment is a constraint
// violation, and lower decides what conversion to emit, and the two must
// agree about what the types are. Where they disagreed, the analyzer would
// accept a program lower could not emit, or reject one it could.

// IsFloat reports whether t is one of §6.2.5's real floating types. The
// complex types are not included: objv does not implement them, and a
// predicate that admitted them would let a later phase believe it could.
func IsFloat(t Type) bool {
	switch Unqualify(t).Kind() {
	case Float16, Float, Double, LongDouble:
		return true
	}
	return false
}

// IsArithmetic is §6.2.5p18: an integer or floating type.
func IsArithmetic(t Type) bool { return IsInteger(t) || IsFloat(t) }

// IsScalar is §6.2.5p21: an arithmetic type or a pointer. A block pointer is
// one: it holds an address, it tests against zero, and it is what `if (block)`
// asks about.
func IsScalar(t Type) bool { return IsArithmetic(t) || IsPointer(t) || IsBlock(t) }

// IsPointer reports whether t is a pointer type. An array is not one until it
// has decayed; see Decay.
func IsPointer(t Type) bool { return Unqualify(t).Kind() == PointerKind }

// IsVoid reports whether t is void.
func IsVoid(t Type) bool { return Unqualify(t).Kind() == Void }

// IsBool reports whether t is _Bool.
func IsBool(t Type) bool { return Unqualify(t).Kind() == Bool }

// IsRecord reports whether t is a struct or a union.
func IsRecord(t Type) bool {
	k := Unqualify(t).Kind()
	return k == StructKind || k == UnionKind
}

// IsFunc reports whether t is a function type.
func IsFunc(t Type) bool { return Unqualify(t).Kind() == FuncKind }

// IsArray reports whether t is an array type.
func IsArray(t Type) bool { return Unqualify(t).Kind() == ArrayKind }

// AsPointer returns t's pointer type, or nil.
func AsPointer(t Type) *Pointer {
	p, _ := Unqualify(t).(*Pointer)
	return p
}

// AsArray returns t's array type, or nil.
func AsArray(t Type) *Array {
	a, _ := Unqualify(t).(*Array)
	return a
}

// AsFunc returns t's function type, or nil.
func AsFunc(t Type) *Func {
	f, _ := Unqualify(t).(*Func)
	return f
}

// AsRecord returns t's struct or union type, or nil.
func AsRecord(t Type) *Record {
	r, _ := Unqualify(t).(*Record)
	return r
}

// Decay applies §6.3.2.1p3-4: an array becomes a pointer to its first
// element, a function becomes a pointer to itself. Everything else is
// unchanged.
//
// The element's qualifiers travel with it — `const char[8]` decays to
// `const char *`, which is what makes assigning it to `char *` a violation.
func Decay(t Type) Type {
	switch u := Unqualify(t).(type) {
	case *Array:
		return &Pointer{Elem: u.Elem}
	case *Func:
		return &Pointer{Elem: u}
	}
	return t
}

// Promote applies §6.3.1.1p2's integer promotions: a type of integer rank
// below int becomes int, or unsigned int where int cannot represent every
// value of the original. Everything else is unchanged.
func (m Model) Promote(t Type) Type {
	t = Unqualify(t)
	if !IsInteger(t) {
		return t
	}
	if e, isEnum := t.(*Enum); isEnum {
		// An enumerated type promotes to the type it is compatible with:
		// int, unless an enumerator too large for one widened it.
		return Typ(e.Underlying())
	}
	intBits, _ := m.IntBits(Typ(Int))
	bits, signed := m.IntBits(t)
	if bits > intBits || (bits == intBits && !signed && !isSmallInt(t)) {
		return t
	}
	if bits < intBits || signed {
		return Typ(Int)
	}
	return Typ(UInt)
}

func isSmallInt(t Type) bool {
	b, ok := Unqualify(t).(*Basic)
	if !ok {
		return false
	}
	switch b.K {
	case Bool, Char, SChar, UChar, Short, UShort:
		return true
	}
	return false
}

// Usual applies §6.3.1.8's usual arithmetic conversions and returns the common
// type both operands convert to.
func (m Model) Usual(a, b Type) Type {
	a, b = Unqualify(a), Unqualify(b)
	if IsFloat(a) || IsFloat(b) {
		return widerFloat(a, b)
	}
	a, b = m.Promote(a), m.Promote(b)
	if a == b {
		return a
	}
	ab, as := m.IntBits(a)
	bb, bs := m.IntBits(b)
	switch {
	case as == bs:
		if ab >= bb {
			return a
		}
		return b
	case !as && ab >= bb:
		return a
	case !bs && bb >= ab:
		return b
	case as && ab > bb:
		return a
	case bs && bb > ab:
		return b
	}
	// Equal width, mixed signedness: the unsigned counterpart of the signed
	// type is the common one.
	if as {
		return UnsignedOf(a)
	}
	return UnsignedOf(b)
}

func widerFloat(a, b Type) Type {
	rank := func(t Type) int {
		bt, ok := Unqualify(t).(*Basic)
		if !ok {
			return 0
		}
		switch bt.K {
		case LongDouble:
			return 3
		case Double:
			return 2
		case Float:
			return 1
		}
		return 0
	}
	if rank(a) >= rank(b) {
		return a
	}
	return b
}

// UnsignedOf returns the unsigned type of the same rank.
func UnsignedOf(t Type) Type {
	b, ok := Unqualify(t).(*Basic)
	if !ok {
		return t
	}
	switch b.K {
	case SChar, Char:
		return Typ(UChar)
	case Short:
		return Typ(UShort)
	case Int, EnumKind:
		return Typ(UInt)
	case Long:
		return Typ(ULong)
	case LongLong:
		return Typ(ULongLong)
	case Int128:
		return Typ(UInt128)
	}
	return t
}

// SizeType is size_t: the unsigned integer of pointer width. There is no Kind
// for it, so it is named by width rather than by spelling — which is also how
// <stddef.h>'s typedef is generated, so the two agree.
func (m Model) SizeType() Type {
	switch m.SizePtr {
	case m.SizeLong:
		return Typ(ULong)
	case m.SizeInt:
		return Typ(UInt)
	default:
		return Typ(ULongLong)
	}
}

// PtrDiffType is ptrdiff_t: the signed integer of pointer width.
func (m Model) PtrDiffType() Type {
	switch m.SizePtr {
	case m.SizeLong:
		return Typ(Long)
	case m.SizeInt:
		return Typ(Int)
	default:
		return Typ(LongLong)
	}
}

// Compatible is §6.2.7p1, within one translation unit.
//
// Identity is what a tag gets: two Records or Enums are the same type exactly
// when they are the same pointer, which is what the analyzer's tag scope
// already guarantees. Everything else is structural.
//
// The one place this is deliberately loose is the unprototyped function.
// §6.7.6.3p15's rules for combining `int f()` with a definition are more than
// a compatibility test can express, and refusing to match it would reject a
// declaration style the standard still admits — so an unprototyped function
// type is compatible with any function type of a compatible return type.
func Compatible(a, b Type) bool {
	if a == nil || b == nil {
		return false
	}
	if QualsOf(a) != QualsOf(b) {
		return false
	}
	return compatUnqual(Unqualify(a), Unqualify(b))
}

// CompatibleIgnoringQuals is Compatible with the outermost qualifiers
// dropped, which is what an assignment's pointee test asks for: §6.5.16.1
// wants the pointed-to types compatible, and checks the qualifiers
// separately because the direction matters.
func CompatibleIgnoringQuals(a, b Type) bool {
	if a == nil || b == nil {
		return false
	}
	return compatUnqual(Unqualify(a), Unqualify(b))
}

func compatUnqual(a, b Type) bool {
	if a == b {
		return true
	}
	// An enumerated type is compatible with its implementation type
	// (§6.7.2.2p4), which this implementation chooses as int unless an
	// enumerator did not fit in one.
	if e, ok := a.(*Enum); ok {
		return b.Kind() == e.Underlying() || b.Kind() == EnumKind && a == b
	}
	if e, ok := b.(*Enum); ok {
		return a.Kind() == e.Underlying()
	}
	if a.Kind() != b.Kind() {
		return false
	}
	switch x := a.(type) {
	case *Basic:
		return x.K == b.(*Basic).K
	case *Pointer:
		return Compatible(x.Elem, b.(*Pointer).Elem)
	case *Object:
		return sameObject(x, b.(*Object))
	case *Block:
		return Compatible(x.Sig, b.(*Block).Sig)
	case *TypeParam:
		return x == b.(*TypeParam)
	case *Array:
		y := b.(*Array)
		if !Compatible(x.Elem, y.Elem) {
			return false
		}
		// A size is compared only where both are known; §6.2.7p1 leaves an
		// incomplete array compatible with a completed one.
		if x.Form == FixedArray && y.Form == FixedArray {
			return x.Len == y.Len
		}
		return true
	case *Func:
		y := b.(*Func)
		if !Compatible(x.Ret, y.Ret) {
			return false
		}
		if !x.Proto || !y.Proto {
			return true // see the note on Compatible
		}
		if x.Variadic != y.Variadic || len(x.Params) != len(y.Params) {
			return false
		}
		for i := range x.Params {
			if !Compatible(AdjustParam(x.Params[i].Type), AdjustParam(y.Params[i].Type)) {
				return false
			}
		}
		return true
	}
	// Records reach here only when they are different pointers, which under
	// tag identity means different types.
	return false
}

// AssignKind classifies the result of an assignment-compatibility test.
type AssignKind uint8

const (
	// AssignOK: §6.5.16.1's constraints are met.
	AssignOK AssignKind = iota
	// AssignBad: the two types have no assignment relation at all.
	AssignBad
	// AssignPointerMismatch: both are pointers, to types that are not
	// compatible. gcc and clang make this a warning by default; §6.5.16.1
	// makes it a constraint violation.
	AssignPointerMismatch
	// AssignDiscardsQuals: the pointee types are compatible, but the target
	// drops a qualifier the source has.
	AssignDiscardsQuals
	// AssignIntPointer: one side is a pointer and the other an integer that
	// is not a null pointer constant.
	AssignIntPointer
	// AssignObjCUnrelated: both are object pointers, of classes with no
	// relation — or with the relation the wrong way round, which is a
	// downcast and needs a cast to say so.
	AssignObjCUnrelated
	// AssignObjCProtocol: the classes are fine, but the source is not known
	// to conform to a protocol the destination requires.
	AssignObjCProtocol
	// AssignObjCTypeArgs: the same generic class, specialized differently.
	AssignObjCTypeArgs
)

// Assignable is §6.5.16.1's constraint list, plus the same rules §6.5.2.2
// applies to an argument and §6.8.6.4 to a return value — the standard defines
// all three by reference to simple assignment.
//
// nullConst says the right operand is a null pointer constant, which the
// caller decides: it is a property of the expression, not of its type.
func Assignable(dst, src Type, nullConst bool) AssignKind {
	l, r := Unqualify(dst), Unqualify(Decay(src))

	switch {
	case IsArithmetic(l) && IsArithmetic(r):
		return AssignOK

	case IsRecord(l):
		if CompatibleIgnoringQuals(l, r) {
			return AssignOK
		}
		return AssignBad

	case IsBool(l) && (IsPointer(r) || IsBlock(r)):
		// §6.5.16.1p1's last bullet: a pointer assigns to _Bool, and a
		// block pointer is one.
		return AssignOK

	// Objective-C's own rules, which are about what a value may be rather
	// than about what it points at.
	case IsObjCObject(l) || IsObjCObject(r):
		// The originals, not the unqualified pair: __kindof is a qualifier,
		// and it is the one that decides whether a downcast needs a cast.
		return objcAssignable(dst, Decay(src), nullConst)

	case IsPointer(l) && nullConst:
		return AssignOK

	case IsPointer(l) && IsPointer(r):
		lp, rp := AsPointer(l), AsPointer(r)
		le, re := lp.Elem, rp.Elem
		// A pointer to void converts to and from a pointer to any object
		// type, but not to a pointer to a function.
		voidEither := (IsVoid(le) && !IsFunc(re)) || (IsVoid(re) && !IsFunc(le))
		if !voidEither && !CompatibleIgnoringQuals(le, re) {
			return AssignPointerMismatch
		}
		// The target must carry every qualifier the source's pointee has.
		if QualsOf(re)&^QualsOf(le) != 0 {
			return AssignDiscardsQuals
		}
		return AssignOK

	case IsPointer(l) && IsInteger(r), IsInteger(l) && IsPointer(r):
		return AssignIntPointer
	}
	return AssignBad
}

// ---- Objective-C ----

// sameObject is compatibility for interface types: the same class, the same
// specialization, and the same protocols.
//
// Type arguments are invariant. `NSArray<NSString *> *` and
// `NSArray<NSMutableString *> *` are different types even though every
// element of the second is an element of the first, because the array is
// writable: a covariant rule would let a caller put an NSString into an
// array the callee believes holds NSMutableStrings. §5.5's __covariant is
// how a class opts out of that, and it is checked at assignment rather than
// here — this is the question of whether two types are the same, and they
// are not.
func sameObject(a, b *Object) bool {
	if a.Base != b.Base || a.Meta != b.Meta {
		return false
	}
	if len(a.Args) != len(b.Args) {
		// An unspecialized type is compatible with a specialized one:
		// `NSArray *` and `NSArray<NSString *> *` are the same class, and
		// erasing the arguments is what the runtime does anyway.
		if len(a.Args) != 0 && len(b.Args) != 0 {
			return false
		}
	} else {
		for i := range a.Args {
			if !Compatible(a.Args[i], b.Args[i]) {
				return false
			}
		}
	}
	return sameProtocols(a.Protocols, b.Protocols)
}

func sameProtocols(a, b []*Protocol) bool {
	if len(a) != len(b) {
		return false
	}
	for _, p := range a {
		if !conformsAny(b, p) {
			return false
		}
	}
	return true
}

// objcAssignable is the assignment rule for object and block pointers.
//
// The shape of it is the language's: `id` converts to and from every object
// pointer, a subclass converts to its superclass, a protocol qualifier is a
// promise the source has to keep, and everything else needs a cast. What is
// deliberately absent is any consultation of ARC — whether a conversion
// between an object pointer and a void * needs a bridge is a question about
// the ownership model in force, which the analyzer knows and this package
// does not.
func objcAssignable(l, r Type, nullConst bool) AssignKind {
	if nullConst {
		return AssignOK
	}
	lb, rb := AsBlock(l), AsBlock(r)
	lo, ro := AsObject(l), AsObject(r)

	// A type parameter is erased, so it assigns to and from any object
	// pointer: what §5.5 checks is the specialization, at the point where
	// the arguments are written, and by here the parameter is `id` in all
	// but name.
	if AsTypeParam(l) != nil || AsTypeParam(r) != nil {
		if IsObjCObject(l) && IsObjCObject(r) {
			return AssignOK
		}
		return AssignPointerMismatch
	}

	switch {
	case lb != nil && rb != nil:
		// Two blocks: compatible signatures, or a cast.
		if Compatible(lb.Sig, rb.Sig) {
			return AssignOK
		}
		return AssignPointerMismatch

	case lo != nil && rb != nil:
		// A block is an object: it assigns to id, and to a class only if
		// that class is one a block could be.
		if lo.Base == nil && !lo.Meta {
			return AssignOK
		}
		return AssignPointerMismatch

	case lb != nil && ro != nil:
		// The other direction needs a cast unless the source is id, which
		// might be a block.
		if ro.Base == nil && !ro.Meta {
			return AssignOK
		}
		return AssignPointerMismatch

	case lo != nil && ro != nil:
		return objectAssignable(l, r, lo, ro)

	// An object pointer and a void * convert freely as types. Under ARC one
	// of the three bridge casts of §6.5 is required, which is the
	// analyzer's rule and not this one.
	case lo != nil && IsPointer(r) && IsVoid(AsPointer(r).Elem):
		return AssignOK
	case ro != nil && IsPointer(l) && IsVoid(AsPointer(l).Elem):
		return AssignOK
	case lb != nil && IsPointer(r) && IsVoid(AsPointer(r).Elem):
		return AssignOK
	case rb != nil && IsPointer(l) && IsVoid(AsPointer(l).Elem):
		return AssignOK

	case IsBool(Unqualify(l)):
		return AssignOK // an object pointer tests like any other

	case IsInteger(l) || IsInteger(r):
		// An object pointer and an integer, which is §6.5's cast or a
		// mistake — and never a null pointer constant, which the first
		// line of this function has already answered.
		return AssignIntPointer
	}
	return AssignPointerMismatch
}

func objectAssignable(l, r Type, lo, ro *Object) AssignKind {
	// `Class` and an instance pointer are different things, and `id` is the
	// only instance type a class object converts to — it is the type that
	// says nothing is known, and a Class is something.
	if lo.Meta != ro.Meta {
		if (lo.Meta && ro.Base == nil && len(ro.Protocols) == 0) ||
			(ro.Meta && lo.Base == nil && len(lo.Protocols) == 0) {
			return AssignOK
		}
		return AssignObjCUnrelated
	}

	// id is the universal object type: it converts both ways, and the
	// protocol qualifiers on the destination are the only thing left to
	// check.
	switch {
	case lo.Base == nil || ro.Base == nil:
		// Nothing to say about the classes.
	case ro.Base.IsSubclassOf(lo.Base):
		// A subclass is its superclass.
	case IsKindOf(r) && lo.Base.IsSubclassOf(ro.Base):
		// __kindof Super * holds any subclass, so it assigns to one
		// without a cast — which is what the qualifier is for.
	default:
		return AssignObjCUnrelated
	}

	// The type arguments must agree where both sides have them.
	if len(lo.Args) > 0 && len(ro.Args) > 0 {
		if len(lo.Args) != len(ro.Args) {
			return AssignObjCTypeArgs
		}
		for i := range lo.Args {
			if !Compatible(lo.Args[i], ro.Args[i]) && !isVariantArg(lo.Base, i, lo.Args[i], ro.Args[i]) {
				return AssignObjCTypeArgs
			}
		}
	}

	// Every protocol the destination names, the source must conform to. An
	// id with no class and no protocols is exempt: it is the type that says
	// nothing is known, and the language lets it convert to anything.
	if ro.Base == nil && len(ro.Protocols) == 0 {
		return AssignOK
	}
	for _, p := range lo.Protocols {
		if !ro.Conforms(p) {
			return AssignObjCProtocol
		}
	}
	return AssignOK
}

// isVariantArg reports whether §5.5's variance lets a type argument differ.
//
// __covariant admits a subtype where the parameter appears — an
// NSArray<NSMutableString *> * is an NSArray<NSString *> * — and
// __contravariant admits a supertype. Without either the parameter is
// invariant, which is the default and the safe one.
func isVariantArg(c *Class, i int, want, have Type) bool {
	if c == nil || i >= len(c.TypeParams) {
		return false
	}
	wo, ho := AsObject(want), AsObject(have)
	if wo == nil || ho == nil || wo.Base == nil || ho.Base == nil {
		return false
	}
	switch c.TypeParams[i].Variance {
	case Covariant:
		return ho.Base.IsSubclassOf(wo.Base)
	case Contravariant:
		return wo.Base.IsSubclassOf(ho.Base)
	}
	return false
}

// BridgeKind says what a conversion between an object pointer and a
// non-object pointer requires under ARC.
type BridgeKind uint8

const (
	// BridgeNone: the conversion involves no object pointer, or both sides
	// are object pointers, so ownership does not change hands.
	BridgeNone BridgeKind = iota
	// BridgeNeeded: one side is an object pointer and the other is not, so
	// §6.5's __bridge, __bridge_retained or __bridge_transfer has to say
	// what happens to the ownership of the value.
	BridgeNeeded
)

// Bridge reports whether converting src to dst crosses the boundary ARC
// manages. It is a fact about the two types; whether crossing it without a
// keyword is an error depends on whether ARC is on, which the analyzer
// knows.
func Bridge(dst, src Type) BridgeKind {
	l, r := IsObjCObject(dst), IsObjCObject(src)
	if l == r {
		return BridgeNone
	}
	other := dst
	if l {
		other = src
	}
	if IsPointer(other) || IsInteger(other) {
		return BridgeNeeded
	}
	return BridgeNone
}
