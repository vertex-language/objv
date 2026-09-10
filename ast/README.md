# ast

`package ast` defines the syntax tree objv's parser builds.

```
import "github.com/vertex-language/objv/ast"
```

Four hierarchies — `Expr`, `Stmt`, `Decl`, `Declarator` — over the whole of
[`docs/objc_grammar.md`](../docs/objc_grammar.md) §3–§7. Declarators are
first-class because in C the declarator *is* the type syntax, and Objective-C
changes that only by adding one constructor to it (the block pointer, §5.7).
Declaration specifiers implement `Expr` so that a specifier list is one
ordered slice (`DeclSpecs`); they never appear in expression position — the
parser doesn't build such trees.

Objective-C's own constructs are in the same four hierarchies. A
`ClassInterfaceDecl` is a `Decl`, a `MessageExpr` is an `Expr`, a `TryStmt` is
a `Stmt`. Nothing about the language needs a fifth family.

## Invariants

**Every node embeds a `Span`.** `Pos` and `End` are stored, not derived, so
even error-recovery nodes have a real, non-empty extent — except at end of
file, where a node built from nothing has nothing left to underline.

**Nodes hold no text.** An `Ident` is two positions; a literal is two
positions and a `token.Kind`. Decoding, escape interpretation, and §6.1's
string concatenation belong to phases above this one.

**The tree is what was written.** Property dot syntax is a `MemberExpr`, not
the message send it becomes. A subscript is an `IndexExpr`, not
`objectForKeyedSubscript:`. `@42` is a `BoxedExpr`, not a `+numberWithInt:`
send. Those rewrites need types and belong to the analyzer; a diagnostic
pointing at code the user did not write would be worse than no diagnostic.

```go
type Node interface {
	Pos() token.Pos // first byte
	End() token.Pos // one past the last byte
}
```

Marker methods are unexported, so the hierarchies are closed.

## Usage

```go
unit := token.NewFile("main.m", src)
file, diags := parser.ParseFile(unit, parser.DefaultMode)
defer file.Release()

ast.Inspect(file, func(n ast.Node) bool {
	if cls, ok := n.(*ast.ClassInterfaceDecl); ok {
		p := unit.Position(cls.Name.Pos())
		fmt.Printf("%d:%d\t@interface %s\n", p.Line, p.Column, cls.Name.Name(unit))
	}
	return true
})
```

Since the tree holds no strings, anything reading spelling takes the
`*token.File`: `ident.Name(f)`, `f.Slice(lit.Lo, lit.Hi)`,
`ast.Fdump(w, f, n)`.

## `File`

`File.Decls` holds §3's external declarations in written order — a `FuncDecl`
or `GenDecl`, one of the six class and protocol forms, a forward declaration,
a `CompatAliasDecl`, an `ImportDecl`, a file-scope `AsmStmt`, or an
`EmptyDecl` for a stray semicolon. `File.Unit` is the position space every
span resolves through. `File.Comments` retains comment tokens when the parser
runs with `parser.ParseComments`.

`File.Release` returns the parser's arena. Every node is invalid afterwards;
copy what you need first.

## Objective-C in four hierarchies

### Declarations

| node | grammar |
| --- | --- |
| `ClassInterfaceDecl` | §4.1 `@interface … @end` |
| `ClassImplDecl` | §4.1 `@implementation … @end` |
| `CategoryDecl` | §4.2 category **and** class extension (`IsExtension`) |
| `CategoryImplDecl` | §4.2 `@implementation C (Cat)` |
| `ProtocolDecl` | §4.3 `@protocol … @end` |
| `ClassForwardDecl`, `ProtocolForwardDecl` | §4.4 `@class A, B;` / `@protocol A, B;` |
| `CompatAliasDecl`, `ImportDecl` | §4.4 `@compatibility_alias`, `@import` |
| `IvarList`, `VisibilityDecl` | §4.5 instance variables and `@private` … |
| `MethodDecl`, `KeywordDecl`, `MethodType`, `ProtoQual` | §4.7 methods |
| `PropertyDecl`, `PropertyAttr`, `PropertyImplDecl` | §4.8 properties |
| `RequirementDecl` | `@required` / `@optional` |

A **category and a class extension are one node**, because they are one
production with one part omitted: `CategoryDecl.Name` is nil for an
extension. What differs between them is what they may contain, which is a
rule and not a shape.

**Markers keep their place in the list.** `@required`, `@optional`,
`@private` and friends are list items (`RequirementDecl`, `VisibilityDecl`)
rather than a field on each member. That is what the grammar says — §4.3
makes a section a run of declarations after a marker — and it keeps written
order intact, which is what a formatter needs and what lets the analyzer
report a marker written where §4.3 does not allow one.

`MethodDecl` is both the declaration and the definition; `Body` says which.
Attributes are kept in **three** fields because §4.7 puts them in three
positions that attach to three different things: before the selector, inside
a keyword between its type and its parameter name, and after the complete
selector — the last being where `NS_DESIGNATED_INITIALIZER` and the
deprecation macros land.

### Expressions

| node | grammar |
| --- | --- |
| `MessageExpr`, `KeywordArg`, `SuperExpr`, `ClassExpr` | §6.3 message sends |
| `SelectorExpr`, `ProtocolExpr`, `EncodeExpr` | §6.4 `@selector`, `@protocol`, `@encode` |
| `BoxedExpr`, `ArrayLit`, `DictLit` | §6.8 object literals |
| `BlockLit` | §6.9 block literals |
| `AvailabilityExpr` | §6.10 `@available` and `__builtin_available` |
| `CastExpr` with `Bridge` | §6.5 `__bridge` casts |
| `StringLit` with `Object` | §2.4 / §6.1 `@"…"` sequences |

`MessageExpr` holds a unary send in `Sel` and a keyword send in `Args`;
nothing joins the pieces into `setObject:forKey:` here, because that is a
string and this package holds none.

`SuperExpr` is an `Expr` because it stands where a receiver stands, and it is
a receiver only — `super` names no value and has no type. `ClassExpr` exists
only for §6.3's `ClassName TypeArgumentList` receiver; a bare class name is an
`Ident` like any other identifier resolved by lookup.

C's member access is `MemberExpr` here, so that `SelectorExpr` can be
`@selector(…)`. In Objective-C the second is far more often what someone means
by "selector".

### Statements

`ForInStmt` (§7.1), `TryStmt` with `CatchClause` and `FinallyClause` (§7.2),
`ThrowStmt`, `SyncStmt`, `AutoreleasePoolStmt` (§7.3).

Fast enumeration is **not** a `ForStmt` with an extra field. A C `for` has
three clauses and two semicolons; this has none of them, and nil fields would
invite every consumer to ask whether they are there.

### Types

`ObjectType` is §5.4's `ObjectTypeSpecifier` — `id`, `Class`, `instancetype`,
a class name, a type parameter — with the two angle-bracket lists it may
carry. It is not `TypedefType` even though `id` and `Class` are typedef names
from `<objc/objc.h>`, because only these accept `<NSCopying>` and
`<NSString *>`. `ObjectKind` records which alternative it is, so the lookup
the parser did is not repeated below it.

`ProtocolRefList` and `TypeArgList` are separate nodes for two lists that are
the same tokens: `<A, B>` is one or the other depending on what `A` and `B`
resolve to (§4.1, §5.5). By the time one exists, that resolution has happened.

`BlockPtrDeclarator` is `^` where `PtrDeclarator` is `*`. Two nodes, because
they are two type constructors: a block pointer points at a closure with a
layout and a runtime descriptor, and unlike a pointer it may not be
dereferenced.

## `Tokens`: what stays unparsed

Three constructs take §8's `BalancedTokenSequence`, and each keeps its tokens
rather than a tree:

- an `Attr`'s argument list (§5.9) — `availability(macosx, introduced=10.12.1)`
  parses as no expression, and it is on nearly every declaration in the SDK;
- a `PtrauthSpec`'s arguments (§5.6);
- an `AsmStmt`'s interior (§7.4), whose operand syntax varies by target.

In all three the accepted contents are the implementation's, not the
language's, so a tree shape here would be a claim this package is not entitled
to make. The tokens carry no text either — they are spans in the same `File` —
and an attribute that wants a value reads it back out of them.

## `AsmStmt` is a statement and a declaration

§3 lists an `AsmStatement` among the external declarations, so `*AsmStmt`
implements both marker methods. At file scope it defines whatever it defines
to the linker, not to this translation unit, which is why the node has no name
field: giving it one would claim knowledge of the text.

## Walking

```go
ast.Walk(v, n)                      // Visitor
ast.Inspect(n, func(n ast.Node) bool { … })
```

Children are discovered by reflection over exported fields in source order, so
a new field traverses without `walk.go` changing — which matters in a tree this
wide, where the alternative is ninety switch arms that fall out of date one at
a time. Typed nils are skipped, as are `Span` fields, `[]Span` fields, and
fields tagged `ast:"-"`.

`ast:"-"` marks the two fields that are aliases rather than children:
`FuncDecl.Name` (the same `*Ident` the declarator holds) and `Tokens.List`.
Walking either would visit a node twice or turn data into children.

## Dumping

`ast.Fdump(w, f, n)` prints one node per line, indented by depth, each with
its raw `line:column` — so a dump lines up with what the user typed, even
through trigraphs and splices. Names and literals print their resolved text;
nodes with alternatives print which one they are.

```
MethodDecl 1:1 -
  MethodType 1:3
    TypeName 1:4
      KeywordSpec 1:4 void
  KeywordDecl 1:9
    Ident 1:9 greet
    MethodType 1:15
      TypeName 1:16
        ObjectType 1:16 class
          Ident 1:16 NSString
        PtrDeclarator 1:25
    Ident 1:27 name
```

That is what `objv ast` prints.

## What the grammar does not have

The tree is closed over the grammar, and a few things a C compiler's AST
usually carries are deliberately absent because §7 and §5.10 do not admit
them: computed `goto` and `&&label`, `case lo ... hi` ranges, and
`[lo ... hi]` designator ranges. `__builtin_offsetof` and its kin are not
nodes either — they are calls until the analyzer says otherwise.

One node **is** here that the grammar does not list: `InitDeclarator.AsmLabel`,
the `__asm("_name")` written after a declarator. Darwin's `<sys/cdefs.h>`
defines `__DARWIN_ALIAS` with it and applies it to most of libc, so a compiler
that cannot read one cannot read `<stdio.h>`. It is flagged here rather than
added quietly: the grammar is the specification, and this is a gap in it.

## Dependencies

Imports [`token`](../token) and nothing else in the tree — not `scanner`, not
`preprocessor`. A tree is a shape over a position space; how the tokens got
there is not its business.

Imported by `parser`, which builds it, and by `analyzer`, `lower` and anything
doing static analysis.
