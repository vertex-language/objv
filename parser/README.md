# parser

`package parser` turns a `*token.File` into an `*ast.File` plus a sorted
diagnostic slice.

```
import "github.com/vertex-language/objv/parser"
```

Recursive descent for declarations, statements and Objective-C's own
constructs; precedence climbing for expressions; and one name table instead
of any rollback machinery.

```go
unit := token.NewFile("main.m", src)
file, diags := parser.ParseFile(unit, parser.DefaultMode)
defer file.Release()
```

`ParseFile` runs the scanner itself. Diagnostics from phases 1–2, scanning
and parsing arrive merged and sorted. Input is expected to be preprocessed
Objective-C — a `#` that reaches the parser is consumed as a line and does
not cascade.

**The tree is never nil.** Every entry point returns a node, a `Bad*`
placeholder if it must, so consumers read a tree rather than a success flag.

## Mode

| | |
| --- | --- |
| `ParseComments` | retain comment tokens on the `File` |
| `SkipBodies` | consume function and method bodies balanced, not parsed — every declaration, prototype, typedef and `@interface` still lands |
| `Tolerant` | keep going past the resync budget; for editors, wasteful in batch builds |

## The three ambiguities

The grammar has exactly three places where the same tokens parse two ways,
and it names all three. Each is resolved by what a name *means*, which is why
this parser needs no backtracking and no speculative parse.

**`(T) - x`** is a cast if `T` is a type name and a subtraction otherwise
(§6.5). One table lookup.

**`Foo<Bar>`** is a generic specialization if `Bar` is a type and a protocol
conformance if `Bar` is a protocol (§4.1, §5.5). Both lists may appear, in
that order:

```objc
NSArray<NSString *> <NSCopying> *both;
```

§4.1's rule is to resolve each identifier, and the first one settles it — but
there are three cases rather than two, because a name may be neither yet. A
protocol used before its declaration is still a protocol, so an unknown name
falls back to *shape*: a protocol list is bare names and commas and nothing
else is.

The `<` immediately after a class name in an `@interface` is the same
ambiguity from the other side, and resolves the other way: a variance keyword
or a bound settles it outright, a list whose names are all declared protocols
is a conformance, and anything else is a type parameter list — `@interface
Container<T>` declares a parameter. That is clang's resolution too, and it is
why a protocol must be declared before the class that conforms to it.

An `@implementation` gets no type parameter list of its own (§4.1 erases
generics), but the methods it defines are written in terms of the ones the
interface declared, so the parser remembers each generic class's parameters
and puts them back in scope. clang injects them the same way.

**`[a b]` versus `a[b]`** is settled by where the bracket is, not by what is
in it (§6.2, §6.3): a `[` that opens an expression is a message send, and one
that follows an expression is a subscript. That is the whole rule, and it is
why object subscripting could be added to the language at all.

### The name table

Five kinds — ordinary, typedef, class, protocol, type parameter — in a scope
stack that follows scope exactly, including immediate visibility after a
declarator: names are declared before their initializers parse.

Classes and protocols are entered at *file* scope from wherever they are
declared, because that is what they are: `@class Forward;` inside an
`@implementation` names a class for the rest of the unit. A generic class's
type parameters are entered in a scope of their own, opened at `@interface`
and closed at `@end`, so `T` names a type inside the class and nothing
outside it.

§2.2's implicitly available names — `id`, `Class`, `SEL`, `IMP`, `BOOL`,
`instancetype`, and `Protocol` — are predeclared. They are typedefs in
`<objc/objc.h>` and a forward-declared class, and a real translation unit
imports that header; but a parser that only knows them when the header was
read cannot parse a fragment, and no program may redefine any of them.

## `>>` closes two lists

```objc
NSArray<id<NSCopying>> *elements;
```

The scanner munches `>>` into one token, because nothing below the parser
knows an angle-bracket list is open (§6.6). The parser knows: `expectRangle`
takes the leading `>` with `token.Token.SplitAngle` and **writes the
remainder back into the token stream** for the outer list to close with. Both
halves keep exact spans, so a diagnostic still underlines the right
character. `>>=` and `>=` split the same way.

## Contextual keywords

§2.2's contextual keywords lex as identifiers and mean something only where
they stand, so the parser recognizes each by spelling in exactly one place:

| | |
| --- | --- |
| `in` | after a `for`'s loop variable, which is what makes it fast enumeration (§7.1) — and an ordinary variable name everywhere else, `for (int i = in; …)` included |
| `super` | as a receiver, and nowhere else: it names no value and has no type |
| `instancetype` | as a type, valid only as a method's return type |
| `in out inout bycopy byref oneway` | inside a `MethodType`'s parentheses (§4.7) |
| `nonnull nullable null_unspecified` | inside a `MethodType` too, where §5.6 makes the underscore-free spellings mean the qualifier |
| `getter setter class direct atomic …` | inside a property attribute list (§4.8), whose set is closed — an identifier that is none of them is a syntax error, which is what §4.8 says it is |

§4.7 lets **any** reserved word be a selector piece, so `- (void)default:(int)x;`
and `obj.class` are ordinary. The one exclusion is `__attribute__`, which the
grammar removes from `Selector` so that

```objc
- (instancetype)init __attribute__((objc_designated_initializer));
```

is a unary selector with a tail attribute rather than a method named `init`
taking one. That exclusion is a one-line rule with a test, and without it
every designated initializer in the SDK parses wrong.

## Unknown type names

An identifier standing where only a type may stand — followed by another
identifier, a `*`, a `^`, or a `<` — is a class whose declaration has not
been read:

```objc
NSArray<NSString *> *keys;   // with no @class NSArray
```

The parser says `unknown type name 'NSArray'`, treats it as a class, and
parses the rest of the declaration. Reading it as the declarator instead
would give three diagnostics, none of them naming the cause, and would lose
the generic list. clang recovers the same way.

## What the parser does not do

It interprets nothing beyond names. Literals stay undecoded, selectors are
pieces rather than strings, and constraints are checked where constraints are
checked — a `@try` with neither `@catch` nor `@finally` parses, a category
with instance variables parses, `@defs` parses. The parse exists so that the
diagnostic can name what was written instead of reporting a surprised brace.

The tree is what was written, too: property dot syntax is a `MemberExpr` and
not the getter send it becomes, `@42` is a `BoxedExpr` and not
`+numberWithInt:`. Those rewrites need types.

## Recovery

One mistake is one diagnostic. `errHere` reports at the current token and
then goes quiet until a token is consumed, and never reports twice at one
position. `advanceTo` resyncs to a follow set, stepping over balanced bracket
groups; past 100 attempts the parser goes silent and runs to EOF, unless
`Tolerant`.

Every loop that builds a list checks that the cursor moved, and forces a
resync if it did not, so a production that consumes nothing cannot spin.
Nesting is capped at 1000 for declarators, statements, types and
expressions — a bound that no real source reaches and that turns a
pathological file into one diagnostic.

## Tests

`parser_test.go` asserts shapes: that the three ambiguities resolve the way
the grammar says, that a nameless selector piece survives, that a tail
attribute is not a selector, that recovery keeps the declarations after a
mistake.

Every program in [`tests`](../tests) is parsed on its way to being built and
compared with clang's build of the same file.

## Dependencies

Imports [`token`](../token), [`scanner`](../scanner) and [`ast`](../ast).
Not `preprocessor`: the parser reads a translation unit that phase 4 has
already produced, and a `#` in its input is a mistake it consumes rather than
a directive it executes.
