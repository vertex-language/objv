package ast

import "github.com/vertex-language/objv/token"

// BadExpr covers tokens the parser gave up on. Its span is non-empty even
// when nothing was consumed.
type BadExpr struct {
	Span
}

// BasicLit is an undecoded INT_LIT, FLOAT_LIT, CHAR_LIT, or BOOL_LIT: two
// positions and a kind. Slice the span for the raw spelling.
//
// BOOL_LIT is __objc_yes and __objc_no, which YES and NO expand to. §2.3
// makes them constants rather than identifiers precisely so that this node
// can tell a boxed boolean from a boxed integer (§6.8).
type BasicLit struct {
	Span
	Kind token.Kind
}

// StringLit is one §6.1 StringLiteralSequence: one node, one span per piece,
// prefixes and any @ included — `@"a" "b"` is one node with two segments.
// Concatenation and decoding happen above this package.
//
// Object reports whether the sequence denotes a string object rather than an
// array of characters, which §6.1 decides from the first piece alone: a
// sequence that begins with @"…" is an object however its continuations are
// spelled, and one that begins plain may not later acquire an @.
type StringLit struct {
	Span
	Segs   []Span
	Object bool
}

// ParenExpr is ( X ).
type ParenExpr struct {
	Span
	Lparen token.Pos
	X      Expr
	Rparen token.Pos
}

// GenericExpr is a C11 _Generic selection.
type GenericExpr struct {
	Span
	Generic token.Pos
	Lparen  token.Pos
	Ctrl    Expr
	Assocs  []*GenericAssoc
	Rparen  token.Pos
}

// GenericAssoc is one association: TypeName : expr, or default : expr
// (Type nil, Default valid).
type GenericAssoc struct {
	Span
	Type    *TypeName // nil for default
	Default token.Pos // NoPos unless the default association
	Colon   token.Pos
	Value   Expr
}

// IndexExpr is X[Index].
//
// The operator is overloaded (§6.2): on an object pointer it is a send of
// objectAtIndexedSubscript: or objectForKeyedSubscript:, chosen by the
// subscript's type, and on a pointer or array it is ordinary C. The tree does
// not choose — the choice needs types, and the node is what was written.
type IndexExpr struct {
	Span
	X      Expr
	Lbrack token.Pos
	Index  Expr
	Rbrack token.Pos
}

// CallExpr is Fun(Args...).
type CallExpr struct {
	Span
	Fun    Expr
	Lparen token.Pos
	Args   []Expr
	Rparen token.Pos
}

// MemberExpr is X.Sel or X->Sel; Op distinguishes PERIOD from ARROW.
//
// The dot is overloaded the way the subscript is (§6.2): on a structure it
// selects a member, and on an object pointer it is property dot syntax,
// rewritten to a send of the property's getter — or of its setter, when the
// expression is the left operand of a simple assignment. `NSObject.class`
// reaches here too, with X an Ident the analyzer resolves as a class rather
// than as a variable. All three are this node; which one it is takes types.
type MemberExpr struct {
	Span
	X     Expr
	OpPos token.Pos
	Op    token.Kind // PERIOD or ARROW
	Sel   *Ident
}

// IncDecExpr is the postfix X++ or X--. Prefix forms are UnaryExpr.
type IncDecExpr struct {
	Span
	X     Expr
	OpPos token.Pos
	Op    token.Kind // INC or DEC
}

// CompoundLit is ( TypeName ) { InitializerList }.
type CompoundLit struct {
	Span
	Lparen token.Pos
	Type   *TypeName
	Rparen token.Pos
	Init   *InitList
}

// UnaryExpr is a prefix operator: & * + - ~ ! and prefix ++ --.
type UnaryExpr struct {
	Span
	OpPos token.Pos
	Op    token.Kind
	X     Expr
}

// SizeofExpr is sizeof X or sizeof ( TypeName ): exactly one of X and Type is
// non-nil. Lparen/Rparen are NoPos in the operand form.
type SizeofExpr struct {
	Span
	Sizeof token.Pos
	Lparen token.Pos
	Type   *TypeName
	X      Expr
	Rparen token.Pos
}

// AlignofExpr is _Alignof ( TypeName ), or the extension spelling __alignof,
// which §6.5 also allows to take an expression. Exactly one of Type and X is
// non-nil; Lparen and Rparen are NoPos in the unparenthesized operand form.
type AlignofExpr struct {
	Span
	Alignof token.Pos
	Lparen  token.Pos
	Type    *TypeName
	X       Expr
	Rparen  token.Pos
}

// CastExpr is ( TypeName ) X, and ( BridgeKeyword TypeName ) X.
//
// Bridge is NoPos on an ordinary cast and holds the keyword's position on a
// bridge cast, with Op naming which of the three it is. A bridge cast is a
// cast between an object pointer and a non-object pointer that states what
// happens to ownership; it is required only under ARC, and it is the same
// syntactic node because it is the same syntax (§6.5).
type CastExpr struct {
	Span
	Lparen token.Pos
	Bridge token.Pos  // NoPos unless a bridge cast
	Op     token.Kind // BRIDGE, BRIDGE_RETAINED, or BRIDGE_TRANSFER
	Type   *TypeName
	Rparen token.Pos
	X      Expr
}

// IsBridge reports whether the cast carries one of §6.5's bridge keywords.
func (e *CastExpr) IsBridge() bool { return e.Bridge.IsValid() }

// BinaryExpr collapses the precedence tower of §6.6: one node, a token.Kind
// operator (the ten binary levels plus COMMA). Precedence lives in
// token.Precedence.
type BinaryExpr struct {
	Span
	X     Expr
	OpPos token.Pos
	Op    token.Kind
	Y     Expr
}

// CondExpr is Cond ? Then : Else.
// CondExpr is `Cond ? Then : Else`, and GCC's `Cond ?: Else` with Then nil.
//
// The second form yields the condition itself when it is true, evaluating it
// once — which is the whole reason it exists, since `f() ? f() : g()` calls
// f twice. Then is nil rather than a copy of Cond because a copy would be a
// second evaluation to everything downstream.
type CondExpr struct {
	Span
	Cond     Expr
	Question token.Pos
	Then     Expr // nil in `a ?: b`
	Colon    token.Pos
	Else     Expr
}

// AssignExpr is Lhs op= Rhs. Right-associative; kept apart from BinaryExpr
// because its left operand is constrained to a unary expression and it is not
// driven by the precedence table.
type AssignExpr struct {
	Span
	Lhs   Expr
	OpPos token.Pos
	Op    token.Kind // ASSIGN, MUL_ASSIGN, …
	Rhs   Expr
}

// StmtExpr is §6.1's statement expression: ({ ... }) in expression position.
// Its value is that of the last expression statement in the block.
//
// It is here rather than tolerated because there is no other way to write
// what it writes: a macro that must evaluate an argument exactly once, name
// the result, and still be an expression has this and nothing else.
type StmtExpr struct {
	Span
	Lparen token.Pos
	Body   *CompoundStmt
	Rparen token.Pos
}

// InitList is a braced initializer; it is an Expr because §5.10's Initializer
// is either an assignment expression or a braced list. Comma records a
// trailing comma's position (NoPos if absent).
type InitList struct {
	Span
	Lbrace token.Pos
	Items  []*InitItem
	Comma  token.Pos
	Rbrace token.Pos
}

// InitItem is one [Designation] Initializer, designators in written order.
type InitItem struct {
	Span
	Designators []Node    // *IndexDesignator or *FieldDesignator
	Assign      token.Pos // NoPos when no designation
	Value       Expr      // expression or nested *InitList
}

// IndexDesignator is [ ConstantExpression ].
type IndexDesignator struct {
	Span
	Lbrack token.Pos
	Index  Expr
	Rbrack token.Pos
}

// FieldDesignator is . Identifier.
type FieldDesignator struct {
	Span
	Dot  token.Pos
	Name *Ident
}

func (*BadExpr) exprNode()     {}
func (*BasicLit) exprNode()    {}
func (*StringLit) exprNode()   {}
func (*ParenExpr) exprNode()   {}
func (*GenericExpr) exprNode() {}
func (*IndexExpr) exprNode()   {}
func (*CallExpr) exprNode()    {}
func (*MemberExpr) exprNode()  {}
func (*IncDecExpr) exprNode()  {}
func (*CompoundLit) exprNode() {}
func (*UnaryExpr) exprNode()   {}
func (*SizeofExpr) exprNode()  {}
func (*AlignofExpr) exprNode() {}
func (*CastExpr) exprNode()    {}
func (*BinaryExpr) exprNode()  {}
func (*CondExpr) exprNode()    {}
func (*AssignExpr) exprNode()  {}
func (*StmtExpr) exprNode()    {}
func (*InitList) exprNode()    {}
