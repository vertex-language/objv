package ast

import "github.com/vertex-language/objv/token"

// DeclSpecs is a declaration-specifier list in written order.
type DeclSpecs []Expr

// KeywordSpec represents a single keyword specifier or qualifier (storage class,
// basic type, C qualifier, ARC ownership, or nullability).
type KeywordSpec struct {
	Span
	Kind token.Kind
}

// AlignasSpec is _Alignas(TypeName) or _Alignas(ConstantExpression).
type AlignasSpec struct {
	Span
	Alignas token.Pos
	Lparen  token.Pos
	Type    *TypeName
	X       Expr
	Rparen  token.Pos
}

// TypeofType represents typeof(expr) or typeof(type) (§5.3).
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

// ObjectType is §5.4's ObjectTypeSpecifier: `id`, `Class`, `instancetype`,
// a class name, or a type parameter name, with optional type arguments or protocol qualifiers.
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

// ProtocolRefList is `< ProtocolList >` (§4.3): adopted or qualifying protocols.
type ProtocolRefList struct {
	Span
	Langle token.Pos
	Names  []*Ident
	Rangle token.Pos
}

// TypeArgList is `< TypeName {, TypeName} >` (§5.5): generic type arguments.
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

// StructType represents a struct or union specifier.
type StructType struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // STRUCT or UNION
	Name    *Ident     // nil for anonymous
	Lbrace  token.Pos  // NoPos for incomplete form
	Fields  []Decl
	Defs    *DefsSpec
	Rbrace  token.Pos
	Attrs   []*Attr
	Pack    int64 // #pragma pack alignment ceiling in force, or 0
}

// DefsSpec is `@defs ( ClassName )`.
type DefsSpec struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Name    *Ident
	Rparen  token.Pos
}

// Attr is an attribute (__attribute__((…)) or [[…]]).
type Attr struct {
	Span
	Scope  *Ident // nil unless scoped (e.g. clang::)
	Colons token.Pos
	Name   *Ident
	Lparen token.Pos // NoPos when no arguments
	Args   *Tokens
	Rparen token.Pos
}

// AttrSpec wraps attributes in a declaration specifier list or method signature.
type AttrSpec struct {
	Span
	Bracketed bool
	Attrs     []*Attr
}

// EnumDecl represents an enum specifier with optional fixed underlying type (: Base).
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
type InitDeclarator struct {
	Span
	Decl     Declarator
	AsmLabel *StringLit // __asm("_name") symbol rename
	Attrs    []*Attr    // attributes on this declarator
	Assign   token.Pos  // NoPos when no initializer
	Init     Expr       // expression or *InitList
}

// FuncDecl represents a function definition.
type FuncDecl struct {
	Span
	Specs    DeclSpecs
	Decl     Declarator
	Attrs    []*Attr // attributes after declarator
	AsmLabel *StringLit
	Name     *Ident `ast:"-"` // alias into Decl; never nil in valid definition
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
