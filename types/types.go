// Package types represents Objective-C types and constructs them from
// declaration specifiers and declarators.
//
// Types form trees with pointer identity for struct, union, enum, class,
// and protocol types.
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

	// Int128 and UInt128 represent gcc/clang's __int128 extension.
	Int128
	UInt128

	// Float16 is IEEE binary16 (_Float16).
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

	// VectorKind is clang's extended vector (__attribute__((ext_vector_type(N)))).
	VectorKind

	// Objective-C types.
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
// IsVector reports whether t is an extended vector type.
func IsVector(t Type) bool { return Unqualify(t).Kind() == VectorKind }

// AsVector is t as a vector, or nil.
func AsVector(t Type) *Vector {
	v, _ := Unqualify(t).(*Vector)
	return v
}

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
	// QKindOf qualifies an object pointer to admit subclasses (§5.6 __kindof).
	QKindOf
)

// Lifetime is an ARC ownership qualifier (§5.6).
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

// Nullability is a pointer nullability qualifier (§5.6).
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

// WithLifetime applies an ownership qualifier, replacing any already present.
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

// WithoutQual removes C qualifiers, dropping the Qualified wrapper if empty.
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

// Vector is clang's __attribute__((ext_vector_type(N))): N elements of a scalar type.
type Vector struct {
	Elem Type
	Len  int64
}

func (*Vector) Kind() Kind { return VectorKind }

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
	Align    int64 // _Alignas or aligned: a minimum the layout honors; 0 for none
}

// Record is a struct or union type with pointer identity.
type Record struct {
	Union    bool
	Name     string // "" for anonymous
	Fields   []Field
	Complete bool
	Packed   bool  // __attribute__((packed))
	Align    int64 // __attribute__((aligned(n)))
	Pack     int64 // #pragma pack ceiling
}

// MemberAlign returns the alignment a member of natural alignment n is placed at in this record.
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

// Enum is an enumerated type with pointer identity.
type Enum struct {
	Name     string
	Complete bool
	Under    Kind // compatible integer kind, or Invalid for default (int)
	Fixed    bool // true for fixed underlying type (enum E : T)
	Defined  bool // true if brace-enclosed enumerator list was seen
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
func (e *Enum) ConstType() Type {
	if e.Fixed || e.Under != Invalid {
		return e
	}
	return Typ(Int)
}

// IsInteger reports whether t (unqualified) is an integer type; enums count (§6.2.5p17).
func IsInteger(t Type) bool {
	switch Unqualify(t).Kind() {
	case Bool, Char, SChar, UChar, Short, UShort, Int, UInt,
		Long, ULong, LongLong, ULongLong, Int128, UInt128, EnumKind:
		return true
	}
	return false
}

// IsSigned reports whether an integer type is signed (plain char signedness is target-dependent).
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

// String formats the pointer type, omitting '*' for implicit pointer types like id.
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

func (v *Vector) String() string {
	return fmt.Sprintf("%s __attribute__((ext_vector_type(%d)))", v.Elem, v.Len)
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
