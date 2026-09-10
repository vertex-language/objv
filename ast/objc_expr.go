package ast

import "github.com/vertex-language/objv/token"

// MessageExpr is §6.3's `[ Receiver MessageSelector ]`.
//
// The two selector forms are one node. A unary send fills Sel and leaves Args
// empty; a keyword send fills Args and leaves Sel nil. Nothing joins the
// keyword pieces into a selector name here — that is a string, and this
// package holds none; the analyzer builds `setObject:forKey:` from the
// pieces' spellings when it needs one.
type MessageExpr struct {
	Span
	Lbrack token.Pos
	Recv   Expr // an expression, *SuperExpr, or *ClassExpr
	Sel    *Ident
	Args   []*KeywordArg
	Rbrack token.Pos
}

// KeywordArg is one `[Selector] : AssignmentExpression {, AssignmentExpression}`.
//
// Sel is nil where the piece has no name — §4.7 lets a KeywordDeclarator omit
// its Selector, so `[obj a:1 :2]` sends `a::` and the second piece is
// nameless.
//
// Vals is a slice because §6.3 lets the *final* keyword carry the trailing
// arguments of a variadic method: in `[NSArray arrayWithObjects:a, b, nil]`
// one keyword holds three. That the extra values are permitted only on the
// last piece is a constraint, checked where constraints are checked.
type KeywordArg struct {
	Span
	Sel   *Ident
	Colon token.Pos
	Vals  []Expr
}

// SuperExpr is the `super` receiver.
//
// It is an Expr because it stands where a receiver stands, and it is a
// receiver only: `super` names no value, has no type, and may not appear in
// any other expression position. The parser builds it nowhere else, and the
// analyzer rejects it if a tree ever holds one elsewhere.
type SuperExpr struct {
	Span
}

// ClassExpr is a receiver written as a class name with type arguments:
// `[NSArray<NSString *> array]`.
//
// A bare class name needs no node of its own — it is an Ident, resolved by
// lookup like any other — so the parser builds this only for §6.3's
// `ClassName TypeArgumentList` form, which is the one an expression cannot
// otherwise represent.
type ClassExpr struct {
	Span
	Name     *Ident
	TypeArgs *TypeArgList
}

// SelectorExpr is @selector ( SelectorName ) (§6.4).
//
// Parts holds the pieces in written order: one part with a NoPos colon for a
// unary selector, and one part per keyword otherwise. A part's Name may be
// nil, since @selector(a::) is a selector with two nameless pieces after the
// first.
type SelectorExpr struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Parts   []*SelectorPart
	Rparen  token.Pos
}

// SelectorPart is one `[Selector] [:]` of a selector name.
type SelectorPart struct {
	Span
	Name  *Ident
	Colon token.Pos // NoPos in a unary selector
}

// ProtocolExpr is @protocol ( ProtocolName ) (§6.4): the Protocol object for
// a named protocol.
type ProtocolExpr struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Name    *Ident
	Rparen  token.Pos
}

// EncodeExpr is @encode ( TypeName ) (§6.4): the runtime's type string for a
// type, as a character array.
type EncodeExpr struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Type    *TypeName
	Rparen  token.Pos
}

// BoxedExpr is §6.8's boxed expression: @42, @-1, @'c', @__objc_yes, @(x+1).
//
// The parenthesized form fills Lparen and Rparen; the direct forms leave them
// NoPos, and a sign is an ordinary UnaryExpr inside X. The two are one node
// because they mean one thing — a call to the boxing method for X's type —
// and the difference between them is which spellings §2.5's closed directive
// list leaves available.
type BoxedExpr struct {
	Span
	At     token.Pos
	Lparen token.Pos // NoPos in the direct forms
	X      Expr
	Rparen token.Pos
}

// ArrayLit is @[ a, b, c ] (§6.8). Comma records a trailing comma (NoPos if
// absent).
type ArrayLit struct {
	Span
	At     token.Pos
	Lbrack token.Pos
	Elems  []Expr
	Comma  token.Pos
	Rbrack token.Pos
}

// DictLit is @{ k : v, … } (§6.8). Comma records a trailing comma.
type DictLit struct {
	Span
	At     token.Pos
	Lbrace token.Pos
	Pairs  []*KeyValue
	Comma  token.Pos
	Rbrace token.Pos
}

// KeyValue is one `key : value` of a dictionary literal.
type KeyValue struct {
	Span
	Key   Expr
	Colon token.Pos
	Value Expr
}

// BlockLit is §6.9's block literal: ^ [TypeName] [BlockParameters] Body.
//
// Type is the explicit return type and is nil when it was not written, in
// which case the type is inferred from the body's return statements and is
// void when there are none — inference the analyzer does, not this package.
//
// Lparen is NoPos when no parameter list was written at all. That is not the
// same as an empty one: `^{ }` and `^(void){ }` differ in spelling, and
// though §6.9 makes them equivalent for a block literal, the tree records
// what was there.
type BlockLit struct {
	Span
	Caret    token.Pos
	Type     *TypeName
	Lparen   token.Pos
	Params   []*ParamDecl
	Ellipsis token.Pos // NoPos if absent
	Rparen   token.Pos
	Body     *CompoundStmt
}

// AvailabilityExpr is §6.10's @available ( … , * ) and the
// __builtin_available spelling of the same thing, which is usable where the @
// form is not.
//
// Star is the position of the mandatory trailing `*`, which stands for every
// platform not named and makes the check succeed there. It is required by the
// grammar, so a NoPos here is a tree the parser only builds after reporting.
type AvailabilityExpr struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // AT_AVAILABLE or BUILTIN_AVAILABLE
	Lparen  token.Pos
	Specs   []*AvailabilitySpec
	Comma   token.Pos // before the *
	Star    token.Pos
	Rparen  token.Pos
}

// AvailabilitySpec is one `PlatformName VersionTuple`, as in `macOS 10.12.1`.
//
// Version is the tuple's extent rather than a parsed triple: the scanner hands
// over `10.12.1` as one run (§2.3 classifies it FLOAT_LIT, since no constant
// has two dots), and a version is three optional numbers, not a value. The
// analyzer reads the digits back out of the span.
type AvailabilitySpec struct {
	Span
	Platform *Ident
	Version  Span
}

func (*MessageExpr) exprNode()      {}
func (*SuperExpr) exprNode()        {}
func (*ClassExpr) exprNode()        {}
func (*SelectorExpr) exprNode()     {}
func (*ProtocolExpr) exprNode()     {}
func (*EncodeExpr) exprNode()       {}
func (*BoxedExpr) exprNode()        {}
func (*ArrayLit) exprNode()         {}
func (*DictLit) exprNode()          {}
func (*BlockLit) exprNode()         {}
func (*AvailabilityExpr) exprNode() {}
