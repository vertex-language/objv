package ast

import "github.com/vertex-language/objv/token"

// DeclSpecs is a declaration-specifier list in written order — `int static
// const x;` is valid, deprecated placement and all. Constraint checking
// (specifier multisets, at most one storage class) is the type-building
// phase's job.
type DeclSpecs []Expr

// KeywordSpec is any single-keyword specifier or qualifier: a storage class,
// a builtin type specifier, const/restrict/volatile, _Atomic as a qualifier,
// inline, _Noreturn — and Objective-C's own, which are the reason this node
// carries a Kind rather than being several nodes: __block, __kindof, the four
// ownership qualifiers, and the nullability qualifiers.
//
// The nullability qualifiers have an underscore-free spelling too (§5.6),
// valid only inside a MethodType and in a property attribute list. Those lex
// as identifiers, so the parser resolves them by position and stores the kind
// they mean — `nullable` in a method type is NULLABLE here, exactly as
// _Nullable is, because they are the same qualifier.
type KeywordSpec struct {
	Span
	Kind token.Kind
}

// AlignasSpec is _Alignas(TypeName) or _Alignas(ConstantExpression): exactly
// one of Type and X is non-nil.
type AlignasSpec struct {
	Span
	Alignas token.Pos
	Lparen  token.Pos
	Type    *TypeName
	X       Expr
	Rparen  token.Pos
}

// TypeofType is §5.3's typeof: a type specifier naming the type of an
// expression, or of a type name. Exactly one of Type and X is non-nil.
//
// It is here rather than tolerated because Objective-C code cannot be read
// without it: the weak/strong dance around a captured self is written
// `__strong __typeof__(weakSelf) strongSelf = weakSelf;` and has no other
// spelling.
type TypeofType struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Type    *TypeName
	X       Expr
	Rparen  token.Pos
}

// AtomicType is _Atomic(TypeName) — the type-specifier form. The qualifier
// form is a KeywordSpec; _Atomic exists as both.
type AtomicType struct {
	Span
	Atomic token.Pos
	Lparen token.Pos
	Type   *TypeName
	Rparen token.Pos
}

// PtrauthSpec is §5.6's __ptrauth ( BalancedTokenSequence ), a type qualifier
// describing pointer authentication on targets that provide it. Its arguments
// are tokens because the set of accepted ones is the implementation's, not
// the language's.
type PtrauthSpec struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Args    *Tokens
	Rparen  token.Pos
}

// ObjectType is §5.4's ObjectTypeSpecifier: `id`, `Class`, `instancetype`, a
// class name, or a type parameter's name, each optionally qualified.
//
// It is not TypedefType even though `id` and `Class` are typedef names from
// <objc/objc.h>, because only these accept the two angle-bracket lists:
// `id<NSCopying>`, `NSArray<NSString *> *`, `NSArray<NSString *><NSCopying>`.
// Which of the five a bare identifier is takes lookup, which is the parser's;
// Kind records the answer so nothing below has to ask again.
type ObjectType struct {
	Span
	Kind      ObjectKind
	Name      *Ident // nil for ObjectID, ObjectClass and ObjectInstancetype
	TypeArgs  *TypeArgList
	Protocols *ProtocolRefList
}

// ObjectKind names which alternative of §5.4 an ObjectType is.
type ObjectKind uint8

const (
	ObjectID           ObjectKind = iota // id
	ObjectClass                          // Class
	ObjectInstancetype                   // instancetype — a method return type only
	ObjectNamed                          // a ClassName
	ObjectTypeParam                      // a TypeParameterName, inside a generic class
)

func (k ObjectKind) String() string {
	switch k {
	case ObjectID:
		return "id"
	case ObjectClass:
		return "Class"
	case ObjectInstancetype:
		return "instancetype"
	case ObjectNamed:
		return "class"
	case ObjectTypeParam:
		return "type parameter"
	}
	return "ObjectKind(?)"
}

// ProtocolRefList is §4.3's `< ProtocolList >`: the protocols a type or a
// declaration conforms to.
//
// It is a node rather than a bare slice because the angle brackets are
// positions a diagnostic points at, and because a list is distinguishable
// from its absence: `id` and `id<>` are not the same text.
type ProtocolRefList struct {
	Span
	Langle token.Pos
	Names  []*Ident
	Rangle token.Pos
}

// TypeArgList is §5.5's `< TypeName {, TypeName} >`: the arguments to a
// generic class.
//
// A list of identifiers between angle brackets is ambiguous with a
// ProtocolRefList and is resolved by looking each name up (§4.1); by the time
// one of these exists, that resolution has happened.
type TypeArgList struct {
	Span
	Langle token.Pos
	Args   []*TypeName
	Rangle token.Pos
}

// TypeParamList is §5.5's `< TypeParameter {, TypeParameter} >`, the
// parameters of a generic class declaration.
type TypeParamList struct {
	Span
	Langle token.Pos
	Params []*TypeParam
	Rangle token.Pos
}

// TypeParam is `[Variance] Identifier [: TypeName]`.
//
// Bound is a full TypeName, pointer included: `@interface Container<T : NSView *>`.
// Variance is NoPos unless __covariant or __contravariant was written.
type TypeParam struct {
	Span
	Variance    token.Pos
	VarianceKey token.Kind // COVARIANT or CONTRAVARIANT; ILLEGAL if absent
	Name        *Ident
	Colon       token.Pos
	Bound       *TypeName
}

// StructType is a struct-or-union specifier; Kind is STRUCT or UNION. Fields
// is nil for the incomplete form (no brace list); each member is a *FieldDecl
// or a *StaticAssertDecl.
//
// Defs is §5.8's third alternative, `struct { @defs(ClassName) }`, which
// yields a class's instance-variable layout as structure members. It is
// supported only under the legacy runtime, which objv does not target, so it
// parses here and is rejected by diagnosis — the parse exists so the
// diagnostic can say what it is rather than that a brace was unexpected.
type StructType struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // STRUCT or UNION
	Name    *Ident     // nil for anonymous
	Lbrace  token.Pos  // NoPos for the incomplete form
	Fields  []Decl
	Defs    *DefsSpec
	Rbrace  token.Pos

	// Attrs are the attributes written on the specifier itself, in either
	// position §5.9 admits: between the keyword and the tag, and after the
	// closing brace. They belong to the type rather than to a declaration of
	// it, which is what makes `struct __attribute__((packed)) s` packed
	// everywhere s is named.
	Attrs []*Attr
}

// DefsSpec is `@defs ( ClassName )`.
type DefsSpec struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Name    *Ident
	Rparen  token.Pos
}

// Attr is one entry of an attribute list, in either spelling of §5.9:
// __attribute__((…)) or [[…]].
//
// The name is kept as written, both spellings: every attribute may be written
// with two leading and two trailing underscores, so that a macro named
// `packed` cannot break a header that says `__packed__`. Scope is the first
// half of a scoped name, `clang::` in `[[clang::objc_arc]]`, and is nil
// otherwise.
//
// Args are tokens, not expressions, because §5.9 says BalancedTokenSequence
// and means it: `availability(macosx, introduced=10.12.1)` parses as no
// expression, and it is on nearly every declaration in the SDK. An attribute
// that wants a value reads it back out of the tokens.
type Attr struct {
	Span
	Scope  *Ident // nil unless a scoped name
	Colons token.Pos
	Name   *Ident
	Lparen token.Pos // NoPos when the attribute takes no arguments
	Args   *Tokens
	Rparen token.Pos
}

// AttrSpec carries the attributes written among a declaration's specifiers,
// or in one of §4.7's three method positions. It is an Expr because DeclSpecs
// is a list of them; it specifies nothing on its own.
//
// Bracketed records the [[…]] spelling, which matters only to a formatter:
// the two spellings mean the same thing and differ in where they may appear.
type AttrSpec struct {
	Span
	Bracketed bool
	Attrs     []*Attr
}

// EnumDecl is an enum specifier. It sits in specifier position but is named
// for what it does: declare constants. Comma records a trailing comma (NoPos
// if absent); List is nil for the incomplete form.
//
// Base is §5.8's fixed underlying type, the `: NSInteger` of
// `enum Foo : NSInteger`. It is not a nicety: NS_ENUM expands to a bodyless
// specifier carrying one, and every enumeration in the Cocoa headers is
// written with it. A fixed underlying type completes the type at the
// specifier and decides how a value of it is boxed (§6.8).
type EnumDecl struct {
	Span
	Enum   token.Pos
	Name   *Ident    // nil for anonymous
	Colon  token.Pos // NoPos when no fixed underlying type
	Base   *TypeName
	Lbrace token.Pos // NoPos for the bodyless form
	List   []*Enumerator
	Comma  token.Pos
	Rbrace token.Pos
	Attrs  []*Attr
}

// Enumerator is Name [AttributeSpecifierList] [= Value].
type Enumerator struct {
	Span
	Name   *Ident
	Attrs  []*Attr
	Assign token.Pos // NoPos when no initializer
	Value  Expr
}

// TypedefType records the parser's typedef-table match: an identifier used as
// a type specifier. An identifier that names a class, a protocol-qualified
// type or a type parameter is an ObjectType instead.
type TypedefType struct {
	Span
	Name *Ident
}

// TypeName is SpecifierQualifierList [AbstractDeclarator] — the operand of
// casts, sizeof, _Alignof, _Atomic(), _Generic, @encode, and compound
// literals, and the type of a method, a parameter, a type argument, and a
// generic bound.
type TypeName struct {
	Span
	Specs DeclSpecs
	Decl  Declarator // nil when absent entirely
}

func (*KeywordSpec) exprNode()     {}
func (*AttrSpec) exprNode()        {}
func (*AlignasSpec) exprNode()     {}
func (*AtomicType) exprNode()      {}
func (*TypeofType) exprNode()      {}
func (*PtrauthSpec) exprNode()     {}
func (*ObjectType) exprNode()      {}
func (*StructType) exprNode()      {}
func (*EnumDecl) exprNode()        {}
func (*TypedefType) exprNode()     {}
func (*TypeName) exprNode()        {}
func (*ProtocolRefList) exprNode() {}
func (*TypeArgList) exprNode()     {}

// ---- declarations ----

// BadDecl covers a declaration the parser gave up on.
type BadDecl struct {
	Span
}

// GenDecl is one ordinary declaration:
// DeclarationSpecifiers [InitDeclaratorList] ;
type GenDecl struct {
	Span
	Specs DeclSpecs
	List  []*InitDeclarator
	Semi  token.Pos
}

// InitDeclarator is Declarator [= Initializer].
//
// AsmLabel is the `__asm("_name")` written after a declarator: it renames the
// symbol and nothing else — the object keeps its name, type and linkage, and
// only what the linker sees changes. The grammar of §5.7 does not list it,
// but Darwin's <sys/cdefs.h> defines __DARWIN_ALIAS with it and applies it to
// most of libc, so a compiler that cannot read one cannot read <stdio.h>.
type InitDeclarator struct {
	Span
	Decl     Declarator
	AsmLabel *StringLit
	// Attrs are the attributes written on this declarator rather than in the
	// declaration's specifiers. `int a __attribute__((aligned(16))), b;`
	// aligns a and not b, which is why they are here and not there.
	Attrs  []*Attr
	Assign token.Pos // NoPos when no initializer
	Init   Expr      // expression or *InitList
}

// FuncDecl is a function definition:
// DeclarationSpecifiers Declarator [DeclarationList] CompoundStatement.
// KR holds the declaration list of a K&R definition, kept whole.
//
// Name aliases the identifier inside Decl — the same node, not a copy — so
// Walk skips it.
type FuncDecl struct {
	Span
	Specs    DeclSpecs
	Decl     Declarator
	AsmLabel *StringLit
	Name     *Ident `ast:"-"` // alias into Decl; never nil in a valid definition
	KR       []*GenDecl
	Body     *CompoundStmt
}

// FieldDecl is one struct/union or instance-variable declaration:
// SpecifierQualifierList [StructDeclaratorList] ;
type FieldDecl struct {
	Span
	Specs DeclSpecs
	List  []*FieldDeclarator
	Semi  token.Pos
}

// FieldDeclarator is [Declarator] [: ConstantExpression]. A bit-field has a
// valid Colon; an unnamed bit-field has a nil Decl too.
type FieldDeclarator struct {
	Span
	Decl  Declarator
	Colon token.Pos // NoPos unless a bit-field
	Width Expr      // constant-ness is a check, not a shape
}

// StaticAssertDecl is _Static_assert ( ConstantExpression , StringLiteral ) ;
type StaticAssertDecl struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Cond    Expr
	Comma   token.Pos
	Msg     *StringLit
	Rparen  token.Pos
	Semi    token.Pos
}

// EmptyDecl keeps a stray semicolon visible — at file scope, and in the
// member lists of §4.6, both of which admit one.
type EmptyDecl struct {
	Span
	Semi token.Pos
}

func (*BadDecl) declNode()          {}
func (*GenDecl) declNode()          {}
func (*FuncDecl) declNode()         {}
func (*FieldDecl) declNode()        {}
func (*StaticAssertDecl) declNode() {}
func (*EmptyDecl) declNode()        {}
