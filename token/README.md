# token

`package token` defines the lexical vocabulary of Objective-C over a C11
substrate (ISO/IEC 9899:2011) and the per-translation-unit position space
every span in the front end resolves through.

```
import "github.com/vertex-language/objv/token"
```

No scanner, no parser — just what they share: `Kind`, `Token`, `File`,
`Diagnostic`. The vocabulary is the one [`docs/objc_grammar.md`](../docs/objc_grammar.md)
§2 enumerates, and nothing beyond it.

## Invariants

1. **Nothing below the parser interprets.** Tokens carry no text; literals
   arrive undecoded and resolve through the `File` that produced them.
2. **No cross-file address space.** `Pos` is per-`File`. A `Pos` from one
   file is meaningless in another.
3. **Every span is non-empty.** `End > Pos`, including for `ILLEGAL`. The
   scanner's `EOF` token — one zero-width span at `Pos(Size())` — is the one
   deliberate exception.

## Scope

The parser consumes **preprocessed** source. `HASH` and `HASHHASH` exist
because `#` and `##` are punctuators (§2.6), but there is no directive
grammar here; that lives in `preprocessor/`, which drives the scanner in its
`ScanPP` mode.

`@import` (§4.4) is a different thing entirely and *is* in this package: it
is a `Directive`, it has no `#`, and it survives phase 4 to reach the
parser.

What is deliberately **not** here:

| Not a token | Why | Reaches the grammar as |
| --- | --- | --- |
| `id`, `Class`, `SEL`, `IMP`, `BOOL` | typedefs in `<objc/objc.h>` | `TypedefName` (§5.3) |
| `Protocol` | `@class Protocol;` under `__OBJC__` | `ClassName` (§4.1) |
| `nil`, `Nil`, `YES`, `NO` | macros | `YES` and `NO` expand to `BOOL_LIT` |
| `instancetype`, `nonatomic`, `copy`, `getter`, `in`, `oneway`, … | contextual keywords (§2.2) | `Identifier`, given meaning by position |

Typedef-name disambiguation, class names, protocol names, and the contextual
keywords are all scope- or position-dependent and cannot be token
properties. They live in the parser.

## `Pos`

```go
type Pos int32
```

`Pos` is a translated-text offset plus one, so the zero value `NoPos` is
invalid and distinguishable from a real position at offset 0. Fields like a
delimiter that was never written (an omitted `Rparen`, a class extension's
absent `CategoryName`) hold `NoPos`; `Pos.IsValid()` reports whether a `Pos`
is real.

## `File`

`NewFile` runs phases 1–2 before tokenization — trigraph replacement, then
line splicing — and maps translated text back to raw bytes.

```go
src := []byte("in\\\nt x = 1;")
f := token.NewFile("a.m", src)

f.Text()   // "int x = 1;"      translated
f.Source() // "in\\\nt x = 1;"  raw

pos, end := f.Pos(0), f.Pos(3)
f.Slice(pos, end) // "int"       what the scanner read — feed to decoders
f.Raw(pos, end)   // "in\\\nt"   what the user typed — underline in diagnostics
```

`Raw` widens to cover a whole splice or trigraph when a span cuts through
one. `Position` (offset/line/column) is in raw bytes, so diagnostics line up
with what the user typed.

Trigraph replacements are reported at `Warn` severity. A non-empty file that
does not end in a line terminator is fine and scans identically to one that
does; a file whose final newline is consumed by a line splice gets one
warning, since a continuation that continues into nothing is almost always a
truncated file. An empty file gets no diagnostics at all.

Sources with no `?` and no `\` take a fast path where translated offsets are
raw offsets and `Raw` degrades to a plain slice of the source — no mapping
table is built. Any other source builds a byte-for-byte translated→raw
mapping during phase 1–2 translation.

`f.Between(prev, next)` returns the raw trivia (whitespace, comments,
spliced-away bytes) between two tokens — for formatters.

## `Token`

```go
type Token struct {
	Kind  Kind
	Flags Flags
	Pos   Pos // inclusive
	End   Pos // exclusive
}
```

| Flag | Meaning |
| --- | --- |
| `FlagAdjacent` | No whitespace/comment separates this token from the previous. |
| `FlagNLBefore` | A line terminator appeared before this token. |
| `FlagDigraph` | Punctuator was spelled as a digraph (`<:`, `%:`, …). |
| `FlagSpacedAt` | Whitespace separated a directive's `@` from its keyword. |

The parser is adjacency-sensitive in exactly one place — the `::` of a
scoped `AttributeName` (§5.9), which is two `COLON` tokens. Everything else
these flags carry is for diagnostics and formatters.

## `Kind`

`ILLEGAL`, `EOF`, `COMMENT`, `IDENT`, the literal kinds, the punctuators,
the keywords, and the directives.

```go
token.Lookup("@interface")       // token.IDENT — Lookup does not see the @
token.LookupDirective("interface") // token.AT_INTERFACE
token.Lookup("__bridge")         // token.BRIDGE
token.Lookup("NSString")         // token.IDENT — a class name is the parser's call
```

**Literals.** `INT_LIT`, `FLOAT_LIT`, `CHAR_LIT`, `STRING_LIT`,
`OBJC_STRING_LIT`, `BOOL_LIT`. `@"…"` gets its own kind because §6.1 keys on
it: a `StringLiteralSequence` that starts with one denotes a string object
and may continue in either spelling, and one that starts plain may not
later acquire an `@`. `BOOL_LIT` is `__objc_yes` / `__objc_no`, which `YES`
and `NO` expand to; §2.3 makes them constants rather than keywords so that a
boxed boolean is distinguishable from a boxed integer (§6.8).

**Keywords.** The 44 of C11 §6.4.1 (including `_Imaginary`, reserved even
without Annex G), the 17 Objective-C keyword kinds of §2.2, and four
extension kinds (`ASM`, `TYPEOF`, `AUTO_TYPE`, `EXTENSION`). `IsObjCKeyword`
picks out the middle group.

Alias spellings resolve in `Lookup`, at the one place a spelling becomes a
kind, so no consumer of a kind has to know there was more than one way to
write it: `__typeof__` → `TYPEOF`, `__alignof` → `ALIGNOF`, `__thread` →
`THREAD_LOCAL`, `__nullable` → `NULLABLE`. The underscore-free nullability
spellings (`nullable`, `nonnull`, `null_unspecified`) are *not* aliases —
they are contextual, valid only inside a `MethodType` (§4.7) or a
`PropertyAttributeList` (§4.8), and only the parser knows it is in one.

**Directives.** The 26 of §2.5, `AT_INTERFACE` through `AT_AUTORELEASEPOOL`,
each one token spanning the `@` and its keyword. The list is closed, which
is why `@SomeEnumConstant` is ill-formed and a constant must be boxed as
`@(SomeEnumConstant)`.

`IsLiteral`, `IsPunct`, `IsKeyword`, and `IsDirective` classify a `Kind` by
which fenced range of the enum it falls in.

`Kind.String()` returns the exact spelling — `"@interface"`, `"__bridge"`,
`"<<="` — or the class name for kinds with no fixed spelling (`IDENT`,
`INT_LIT`, …); an out-of-range value renders as `Kind(n)` rather than
panicking.

### Two punctuators that are not kinds

`::` (§2.6) has no kind: it is two adjacent `COLON` tokens. Munching the pair
would break `@selector(a::)` — §6.4's `SelectorName` is pieces and colons
with no parameters to name, so a nameless second piece puts two colons
together — and the scoped `AttributeName` that `::` otherwise serves is a C++
import reached only through the bracketed attribute form, where the parser
recognizes it from `FlagAdjacent`.

`>>` has the opposite problem. It *is* one token, because nothing below the
parser knows that `NSArray<id<NSCopying>>` has two of §5.5's angle-bracket
lists open. `SplitAngle` undoes the munch where the parser needs the first
`>` alone:

```go
lead, rest, ok := tok.SplitAngle() // >> → > >   >>= → > >=   >= → > =
```

The halves keep exact spans, so a diagnostic still underlines the right
character.

## Precedence

```go
func (k Kind) Precedence() int // LowestPrec (0) for non-binary operators
```

Covers the ten binary levels of §6.6 plus `COMMA` below them. Assignment and
`? :` are right-associative and not driven by this table, and report
`LowestPrec` like any other non-binary token.

## Diagnostics

```go
type Severity uint8 // Note, Warn, Error
```

Phases 1–2 are the only work here that reports on its own; their diagnostics
live on the `File` via `f.Diagnostics()`, and the scanner and parser merge
them into their own slices as they run. `SortDiagnostics` orders by
position, then extent, then message, stably — so diagnostics from different
phases interleave deterministically once merged.

`Diagnostic.Print(f)` renders one line through the `File` that owns its
span — `name:line:col: severity: message` — in raw (as-typed) coordinates,
since that is what a human reading the diagnostic actually typed.
