// Package types represents Objective-C types and constructs them from
// declaration specifiers and declarators.
//
// Types are trees; struct, union, enum, class and protocol types
// additionally have identity — two *Record values are the same type iff they
// are the same pointer, which is what makes a tag namespace meaningful, and
// two *Class values name the same class on the same terms. Constraint
// checking that the parser deliberately deferred (specifier multisets,
// qualifier placement, static and * in array declarators, function and array
// derivation rules) happens here, during construction, reported through the
// Resolver.
//
// The Objective-C half is in objc.go, and it is deliberately clang's model
// rather than a simpler one: an interface is a type (*Object), and what a
// program calls "an NSString" is a *pointer* to one. `id` is that pointer
// with no class named. Reading `NSString *` as one atomic thing would work
// until the first `__strong NSString *__weak *`, where the two ownership
// qualifiers belong to different levels.
package types

import (
	"fmt"
	"strings"
)

// Kind identifies a type's shape.
type Kind uint8

const (
	Invalid Kind = iota

	Void
	Bool
	Char // plain char: distinct from SChar and UChar (C11 §6.2.5p15)
	SChar
	UChar
	Short
	UShort
	Int
	UInt
	Long
	ULong
	LongLong
	ULongLong

	// Int128 and UInt128 are gcc's __int128, which clang carries on every
	// 64-bit target and Apple's headers use: <mach/arm/_structs.h>
	// declares the NEON register file as __uint128_t __v[32], and nothing
	// narrower describes a 128-bit vector register.
	Int128
	UInt128

	// Float16 is IEEE binary16, which <math.h> declares its half-precision
	// entry points in terms of.
	Float16

	Float
	Double
	LongDouble
	ComplexFloat
	ComplexDouble
	ComplexLongDouble

	PointerKind
	ArrayKind
	FuncKind
	StructKind
	UnionKind
	EnumKind

	// The Objective-C shapes. An ObjectKind is an interface type — what a
	// pointer to it points at — and never the type of a value; BlockKind is
	// already a pointer, as the language's `^` says.
	ObjectKind
	BlockKind
	TypeParamKind
)

// Type is the interface all types implement.
type Type interface {
	Kind() Kind
	String() string
}

// Basic is a builtin arithmetic type or void. Use Typ for the canonical
// singleton of each kind.
type Basic struct{ K Kind }

var basics [ComplexLongDouble + 1]Basic

func init() {
	for k := range basics {
		basics[k].K = Kind(k)
	}
}

// Typ returns the canonical *Basic for a basic kind.
func Typ(k Kind) *Basic { return &basics[k] }

func (b *Basic) Kind() Kind { return b.K }

// IsComplex reports whether t is one of §6.2.5's complex types.
func IsComplex(t Type) bool {
	switch Unqualify(t).Kind() {
	case ComplexFloat, ComplexDouble, ComplexLongDouble:
		return true
	}
	return false
}

// Qual is the set of C qualifiers.
type Qual uint8

const (
	QConst Qual = 1 << iota
	QVolatile
	QRestrict
	QAtomic
	// QKindOf is §5.6's __kindof, which qualifies an object pointer to
	// admit any subclass of the class named while keeping that class's
	// interface for message sends. It is a qualifier and not a flag on the
	// object because that is where it is written and what it does: it
	// changes what may be assigned, not what the type is.
	QKindOf
)

// Lifetime is an ARC ownership qualifier (§5.6). Exactly one applies to an
// object pointer, so it is an enumeration rather than a bit in Qual.
type Lifetime uint8

const (
	LifeNone Lifetime = iota
	LifeStrong
	LifeWeak
	LifeUnsafeUnretained
	LifeAutoreleasing
)

func (l Lifetime) String() string {
	switch l {
	case LifeStrong:
		return "__strong"
	case LifeWeak:
		return "__weak"
	case LifeUnsafeUnretained:
		return "__unsafe_unretained"
	case LifeAutoreleasing:
		return "__autoreleasing"
	}
	return ""
}

// Nullability is §5.6's nullability qualifier. Like Lifetime it is one of a
// set rather than a bit, and like Lifetime it qualifies a pointer rather
// than describing what the pointer points at.
type Nullability uint8

const (
	NullNone Nullability = iota
	NullNonnull
	NullNullable
	NullUnspecified
)

func (n Nullability) String() string {
	switch n {
	case NullNonnull:
		return "_Nonnull"
	case NullNullable:
		return "_Nullable"
	case NullUnspecified:
		return "_Null_unspecified"
	}
	return ""
}

// Qualified wraps a type with qualifiers. Qualify never nests them.
type Qualified struct {
	Q    Qual
	Life Lifetime
	Null Nullability
	T    Type
}

func (q *Qualified) Kind() Kind { return q.T.Kind() }

// Qualify applies C qualifiers, merging with any already present.
// Qualifying with nothing is the identity.
func Qualify(t Type, q Qual) Type {
	if q == 0 {
		return t
	}
	if in, ok := t.(*Qualified); ok {
		return &Qualified{Q: in.Q | q, Life: in.Life, Null: in.Null, T: in.T}
	}
	return &Qualified{Q: q, T: t}
}

// WithLifetime applies an ownership qualifier, replacing any already
// present — a type has one owner or none.
func WithLifetime(t Type, l Lifetime) Type {
	if l == LifeNone {
		return t
	}
	if in, ok := t.(*Qualified); ok {
		return &Qualified{Q: in.Q, Life: l, Null: in.Null, T: in.T}
	}
	return &Qualified{Life: l, T: t}
}

// WithNullability applies a nullability qualifier, replacing any already
// present.
func WithNullability(t Type, n Nullability) Type {
	if n == NullNone {
		return t
	}
	if in, ok := t.(*Qualified); ok {
		return &Qualified{Q: in.Q, Life: in.Life, Null: n, T: in.T}
	}
	return &Qualified{Null: n, T: t}
}

// WithoutQual removes qualifiers, keeping the rest. It is what moving a
// qualifier from one level of a type to another needs — __kindof is written
// on the class and belongs on the pointer — and it drops the wrapper
// entirely when nothing is left in it.
func WithoutQual(t Type, q Qual) Type {
	in, ok := t.(*Qualified)
	if !ok || in.Q&q == 0 {
		return t
	}
	rest := &Qualified{Q: in.Q &^ q, Life: in.Life, Null: in.Null, T: in.T}
	if rest.Q == 0 && rest.Life == LifeNone && rest.Null == NullNone {
		return rest.T
	}
	return rest
}

// Unqualify strips the qualifier wrapper, if any.
func Unqualify(t Type) Type {
	if q, ok := t.(*Qualified); ok {
		return q.T
	}
	return t
}

// QualsOf returns a type's C qualifiers.
func QualsOf(t Type) Qual {
	if q, ok := t.(*Qualified); ok {
		return q.Q
	}
	return 0
}

// LifetimeOf returns a type's ownership qualifier.
func LifetimeOf(t Type) Lifetime {
	if q, ok := t.(*Qualified); ok {
		return q.Life
	}
	return LifeNone
}

// NullabilityOf returns a type's nullability qualifier.
func NullabilityOf(t Type) Nullability {
	if q, ok := t.(*Qualified); ok {
		return q.Null
	}
	return NullNone
}

// IsKindOf reports whether t carries __kindof.
func IsKindOf(t Type) bool { return QualsOf(t)&QKindOf != 0 }

// Pointer is pointer-to-Elem.
type Pointer struct{ Elem Type }

func (*Pointer) Kind() Kind { return PointerKind }

// ArrayForm distinguishes §5.7's four bracket shapes.
type ArrayForm uint8

const (
	FixedArray      ArrayForm = iota // [N], N a constant expression
	IncompleteArray                  // []
	VLA                              // [expr], expr not constant
	StarArray                        // [*]
)

// Array is array-of-Elem. Len is meaningful only for FixedArray. Static
// records a parameter's [static …].
type Array struct {
	Elem   Type
	Form   ArrayForm
	Len    int64
	Static bool
}

func (*Array) Kind() Kind { return ArrayKind }

// Param is one function, block or method parameter.
type Param struct {
	Name string // "" for unnamed
	Type Type
}

// Func is a function type. Proto is false for f() and identifier-list
// declarators, where the parameters are unspecified.
type Func struct {
	Ret      Type
	Params   []Param
	Variadic bool
	Proto    bool
}

func (*Func) Kind() Kind { return FuncKind }

// Field is one struct/union member.
type Field struct {
	Name     string // "" for an unnamed bit-field or anonymous record
	Type     Type
	BitField bool
	Width    int64 // meaningful when BitField
}

// Record is a struct or union type. Identity is the tag: two Records are the
// same type iff they are the same pointer. Complete flips to true when a
// definition supplies the member list.
type Record struct {
	Union    bool
	Name     string // "" for anonymous
	Fields   []Field
	Complete bool

	// Packed drops the padding between members: every one is placed at the
	// next byte rather than at the next offset its own alignment admits,
	// and the record itself aligns to one. It is
	// __attribute__((packed)), which is in every protocol header written
	// for gcc — a wire format is a struct whose layout the protocol chose.
	Packed bool

	// Align, when non-zero, is the alignment __attribute__((aligned(n)))
	// asked for, in bytes. It raises the record's alignment and therefore
	// its size, and it applies whether or not the record is packed: the two
	// attributes answer different questions, one about the members and one
	// about the whole.
	Align int64

	// Pack, when non-zero, is the ceiling #pragma pack put on each member's
	// alignment. A member whose type wants less keeps what it wants; one
	// that wants more is placed at Pack instead.
	Pack int64
}

// MemberAlign is the alignment a member of natural alignment n is actually
// placed at in this record: capped by #pragma pack, and flattened to one by
// __attribute__((packed)).
//
// It is a method rather than a rule spelled at each layout, because there
// are two layouts — the Model's, which answers sizeof, and lower's, which
// places the members — and they have to agree.
func (r *Record) MemberAlign(n int64) int64 {
	switch {
	case r.Packed:
		return 1
	case r.Pack > 0 && n > r.Pack:
		return r.Pack
	}
	return n
}

func (r *Record) Kind() Kind {
	if r.Union {
		return UnionKind
	}
	return StructKind
}

// Enum is an enumerated type; like Record, it has identity.
//
// Under is the integer type the enumeration is compatible with. C17
// §6.7.2.2p4 leaves the choice to the implementation, and this one answers
// int wherever the enumerators fit — so Under is Invalid there and
// Underlying says int.
//
// Fixed records §5.8's `enum E : T`, the fixed underlying type NS_ENUM
// expands to and every enumeration in the Cocoa headers carries. A fixed
// type is not a widening: it is the type the program named, it completes the
// enumeration at the specifier, and it decides how a value of the type is
// boxed (§6.8).
type Enum struct {
	Name     string
	Complete bool
	Under    Kind
	Fixed    bool

	// Defined records that a brace-enclosed enumerator list was seen.
	//
	// It is not Complete. §5.8's fixed underlying type completes the type
	// at the specifier — `enum E : NSInteger;` declares a type an object
	// may be declared of — while leaving the enumeration itself still to
	// be defined, once. NS_ENUM writes both, in that order:
	//
	//	typedef enum E : NSInteger E;
	//	enum E : NSInteger { ... };
	//
	// so a compiler that read Complete as "already defined" would call the
	// second line a redefinition of the first, and no Cocoa header would
	// get past its first enumeration.
	Defined bool
}

func (*Enum) Kind() Kind { return EnumKind }

// Underlying is the integer kind the enumeration is compatible with.
func (e *Enum) Underlying() Kind {
	if e.Under == Invalid {
		return Int
	}
	return e.Under
}

// ConstType is the type an enumeration constant of e has.
//
// §6.4.4.3p2 says int, and this answers int wherever that is a type the
// value fits. Where the enumeration has a fixed underlying type there is no
// such option: NS_ENUM exists precisely so that the constants have the
// enumeration's type, which is what a `switch` over one and an
// NSInteger-typed parameter both depend on.
func (e *Enum) ConstType() Type {
	if e.Fixed || e.Under != Invalid {
		return e
	}
	return Typ(Int)
}

// IsInteger reports whether t (unqualified) is an integer type; enums count
// (§6.2.5p17).
func IsInteger(t Type) bool {
	switch Unqualify(t).Kind() {
	case Bool, Char, SChar, UChar, Short, UShort, Int, UInt,
		Long, ULong, LongLong, ULongLong, Int128, UInt128, EnumKind:
		return true
	}
	return false
}

// IsSigned reports whether an integer type is signed. Plain char's
// signedness is the Model's business, not the type's; it reports false here
// and callers that care ask the Model.
func IsSigned(t Type) bool {
	u := Unqualify(t)
	if e, ok := u.(*Enum); ok {
		return IsSigned(Typ(e.Underlying()))
	}
	switch u.Kind() {
	case SChar, Short, Int, Long, LongLong, Int128:
		return true
	}
	return false
}

// ---- printing, compact, for diagnostics ----

func (b *Basic) String() string {
	switch b.K {
	case Void:
		return "void"
	case Bool:
		return "_Bool"
	case Char:
		return "char"
	case SChar:
		return "signed char"
	case UChar:
		return "unsigned char"
	case Short:
		return "short"
	case UShort:
		return "unsigned short"
	case Int:
		return "int"
	case UInt:
		return "unsigned int"
	case Long:
		return "long"
	case ULong:
		return "unsigned long"
	case LongLong:
		return "long long"
	case ULongLong:
		return "unsigned long long"
	case Int128:
		return "__int128"
	case UInt128:
		return "unsigned __int128"
	case Float16:
		return "_Float16"
	case Float:
		return "float"
	case Double:
		return "double"
	case LongDouble:
		return "long double"
	case ComplexFloat:
		return "float _Complex"
	case ComplexDouble:
		return "double _Complex"
	case ComplexLongDouble:
		return "long double _Complex"
	}
	return "invalid"
}

func (q *Qualified) String() string {
	var b strings.Builder
	for _, e := range [...]struct {
		q Qual
		s string
	}{{QConst, "const "}, {QVolatile, "volatile "}, {QRestrict, "restrict "},
		{QAtomic, "_Atomic "}, {QKindOf, "__kindof "}} {
		if q.Q&e.q != 0 {
			b.WriteString(e.s)
		}
	}
	if q.Life != LifeNone {
		b.WriteString(q.Life.String() + " ")
	}
	b.WriteString(q.T.String())
	if q.Null != NullNone {
		b.WriteString(" " + q.Null.String())
	}
	return b.String()
}

// String writes a pointer the way the language does. That is not always
// with a star: `id`, `Class`, `instancetype` and `id<NSCopying>` are pointer
// types whose spelling has none, because the pointer is inside the name.
// Only a pointer to a named class takes one — `NSString *`.
func (p *Pointer) String() string {
	if o, ok := Unqualify(p.Elem).(*Object); ok && o.Base == nil {
		return p.Elem.String()
	}
	return p.Elem.String() + "*"
}

func (a *Array) String() string {
	switch a.Form {
	case FixedArray:
		return fmt.Sprintf("%s[%d]", a.Elem, a.Len)
	case VLA:
		return a.Elem.String() + "[<vla>]"
	case StarArray:
		return a.Elem.String() + "[*]"
	}
	return a.Elem.String() + "[]"
}

func (f *Func) String() string {
	var b strings.Builder
	b.WriteString(f.Ret.String())
	b.WriteByte('(')
	if !f.Proto {
		b.WriteByte(')')
		return b.String()
	}
	if len(f.Params) == 0 {
		b.WriteString("void")
	}
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Type.String())
	}
	if f.Variadic {
		b.WriteString(", ...")
	}
	b.WriteByte(')')
	return b.String()
}

func (r *Record) String() string {
	kw := "struct"
	if r.Union {
		kw = "union"
	}
	if r.Name == "" {
		return kw + " <anonymous>"
	}
	return kw + " " + r.Name
}

func (e *Enum) String() string {
	if e.Name == "" {
		return "enum <anonymous>"
	}
	return "enum " + e.Name
}
