// Package ast defines the syntax tree objv's parser builds.
//
// Four hierarchies — Expr, Stmt, Decl, Declarator — mirroring the four
// families of docs/objc_grammar.md. Declarators are first-class because in C
// the declarator is the type syntax, and Objective-C changes that only by
// adding one constructor to it (the block pointer, §5.7). Declaration
// specifiers — type specifiers, qualifiers, storage classes, attributes —
// implement Expr so that a specifier list is one ordered slice; they never
// appear in expression position, because the parser does not build such
// trees.
//
// Objective-C's own constructs are declarations, statements and expressions
// like any other, and they are in the same hierarchies: a ClassInterfaceDecl
// is a Decl, a MessageExpr is an Expr, a TryStmt is a Stmt. Nothing about the
// language needs a fifth family.
//
// Invariants:
//
//   - Every node embeds a Span. Pos and End are stored, not derived, so even
//     error-recovery nodes have a real, non-empty extent — with one
//     exception, at end of file, where a node the parser built from nothing
//     has nothing left to underline and its span stays empty.
//   - Nodes hold no text. An Ident is two positions; a literal is two
//     positions and a token.Kind. Decoding, escape interpretation, and §6.1's
//     string concatenation belong to phases above this one. Anything reading
//     spelling takes the *token.File.
//   - The tree is what was written. Property dot syntax is a MemberExpr and
//     not the message send it becomes; a subscript is an IndexExpr and not
//     objectForKeyedSubscript:. Those rewrites are the analyzer's, and a
//     diagnostic that pointed at code the user did not write would be worse
//     than no diagnostic.
package ast

import "github.com/vertex-language/objv/token"

// Node is the interface all tree nodes implement.
type Node interface {
	Pos() token.Pos // first byte
	End() token.Pos // one past the last byte
}

// Span is the stored extent every node embeds.
type Span struct {
	Lo token.Pos // inclusive
	Hi token.Pos // exclusive
}

func (s Span) Pos() token.Pos { return s.Lo }
func (s Span) End() token.Pos { return s.Hi }

// The four hierarchies. Marker methods are unexported, so the hierarchies
// are closed.

type Expr interface {
	Node
	exprNode()
}

type Stmt interface {
	Node
	stmtNode()
}

type Decl interface {
	Node
	declNode()
}

// Declarator is the type-syntax hierarchy. DeclName returns the declared
// identifier, or nil — an abstract declarator is a declarator with a nil
// name.
type Declarator interface {
	Node
	DeclName() *Ident
	declaratorNode()
}

// Ident is two positions; spelling resolves through the File that produced
// it.
//
// A selector piece is an Ident even when it is spelled with a keyword —
// §4.7 admits any reserved word but __attribute__ as a Selector, and
// `- (void)default:(int)x` is a method named default:. Nothing here records
// which token kind the spelling came from, because nothing needs to: the
// piece is its spelling.
type Ident struct {
	Span
}

// Name returns the identifier's spelling from its translation unit.
func (id *Ident) Name(f *token.File) string {
	return string(f.Slice(id.Lo, id.Hi))
}

func (*Ident) exprNode() {}

// Tokens is §8's BalancedTokenSequence: a run of tokens in which brackets,
// parentheses and braces are balanced, kept as tokens rather than parsed.
//
// Three constructs take one, and each for the same reason — what may appear
// inside is decided by the implementation rather than by the language, so a
// tree shape would be a claim this package is not entitled to make:
//
//   - an Attribute's argument list (§5.9), where
//     `availability(macosx, introduced=10.12.1)` is not an expression in any
//     grammar;
//   - a PtrauthQualifier's arguments (§5.6);
//   - an AsmStatement's interior (§7.4), whose operand syntax varies by
//     target.
//
// The tokens carry no text either; they are spans in the same File, and an
// attribute that needs a value reads it back out of them.
type Tokens struct {
	Span
	List []token.Token `ast:"-"` // spans, not children
}

// Releaser is the one-method window through which ast sees the parser's
// arena.
type Releaser interface {
	Release()
}

// File is one translation unit's tree.
//
// Decls holds the external declarations of §3 in written order: a
// FuncDecl or GenDecl, one of the six class and protocol forms, a forward
// declaration, a compatibility alias, an @import, a file-scope AsmStmt, or
// an EmptyDecl for a stray semicolon.
type File struct {
	Span
	Unit     *token.File   // the position space every span resolves through
	Decls    []Decl        // external declarations
	Comments []token.Token `ast:"-"` // retained under parser.ParseComments

	rel Releaser
}

// SetReleaser attaches the tree's backing storage. The parser calls this;
// consumers call Release.
func (f *File) SetReleaser(r Releaser) { f.rel = r }

// Release returns the tree's backing storage. It is safe on a tree with no
// releaser and safe to call twice, but every node is invalid afterwards —
// copy what you need (usually a span and a string) before calling it.
// Release is a promise, not a check: nothing detects a kept pointer.
func (f *File) Release() {
	if f.rel != nil {
		r := f.rel
		f.rel = nil
		r.Release()
	}
}
