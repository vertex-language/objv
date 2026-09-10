# preprocessor

`package preprocessor` is translation phase 4: directive execution, macro
replacement, and header resolution.

```go
import "github.com/vertex-language/objv/preprocessor"
```

C11 §6.10, plus the three things Objective-C adds to it: `#import`, framework
includes, and the clang interrogation operators the Cocoa headers gate every
declaration on. The directive grammar lives here and nowhere else in the tree.
Below this package nothing expands and nothing interprets; above it, on `.mi`
input, this package is skipped entirely and a `#` opening a line is trivia.

`@import` (§4.4 of [the grammar](../docs/objc_grammar.md)) is **not** this
package's. It carries no `#`, survives phase 4 as the `AT_IMPORT` token it was
scanned as, and is the parser's to read.

## Invariants

1. **Pure function of `Config`.** Same config, same input, same output, every
   time. Mounts are `fs.FS`; this package never imports `os`, never probes a
   host, never reads a clock.
2. **One diagnostic per mistake.** A bad directive is reported once. A macro
   arity error is reported at the invocation, not at every token it produced.
   A warning sited in a system header is reported once per header, not once
   per inclusion — which matters more here than in a C compiler, because
   `#import <Foundation/Foundation.h>` reaches some nine hundred files.
3. **Positions survive expansion.** Every token carries the `Origin` its span
   belongs to and, when a macro produced it, the chain of use sites and
   definition sites that made it. A diagnostic in expanded code points at what
   the user typed.
4. **Nothing is decoded.** Literals leave phase 4 exactly as they entered.
   `#if`'s evaluator decodes internally, for itself, and throws the result
   away.

## Usage

```go
cfg := preprocessor.Config{
	Search: []preprocessor.Mount{
		{Name: "include", FS: os.DirFS("include")},
		{Name: sdk + "/usr/include", FS: os.DirFS(sdk + "/usr/include"), System: true},
	},
	Frameworks: []preprocessor.Mount{
		{Name: sdk + "/System/Library/Frameworks", FS: os.DirFS(...), System: true},
	},
	Triple:  preprocessor.Triple{Arch: "arm64", Vendor: "apple", OS: "macos"},
	Feature: func(name string) bool { return arcFeatures[name] },
	Hosted:  true,
}

p := preprocessor.New(cfg)
toks, diags := p.Run(token.NewFile("main.m", src))
```

`Run` returns the token stream phase 5 reads and every diagnostic phases 1
through 4 produced, sorted. The stream is **never nil**; a file that failed to
parse still yields the tokens that were read.

The `os.DirFS` calls belong to the caller, not here. The `objv` package
composes `sysroot.Resolve`, `-I` and `-F` into those two lists, which are the
lists `objv env` prints.

A zero `Config` preprocesses a self-contained file with no headers and no
macros — which is exactly what a test wants, and exactly what a `.m` with no
`#import` is.

## Where a token came from

Phase 4 breaks `token`'s second invariant — *no cross-file address space* — by
construction: its output is one sequence drawn from `main.m`, `NSString.h`,
and everything they reached. A bare `token.Token` is four fields with no file
identity and cannot say where it is.

`Origin` restores it, and carries the inclusion chain as well:

```go
type Token struct {
	Kind  token.Kind
	Flags token.Flags
	Pos   token.Pos
	End   token.Pos

	Origin *Origin    // the position space Pos and End live in
	Hide   *HideSet   // Prosser's blue paint
	Exp    *Expansion // use site, definition site, outward
}
```

`t.Text()` resolves spelling through `Origin`. Nothing carries text.
`t.Site()` is where a diagnostic about the token should point — the outermost
invocation for a macro-produced token, the token's own span otherwise — and
`t.Notes()` walks the definition sites back out.

An `Origin` with a nil `File` is the **generated arena**: the position space
for tokens that `#`, `##` and `_Pragma` built and no file contains.

## `#import`, and reading a file once

Objective-C has no include guards to write because it has a directive that
states the conclusion outright. Four things can say a file need not be read
again, and this package honours all four:

| | says |
| --- | --- |
| `#import <F/H.h>` | at the point of use, unconditionally |
| `#pragma once` | in the file, unconditionally |
| `_Pragma("once")` | the same thing, from a macro |
| `#ifndef G` / `#define G` / … / `#endif` | while `G` is defined |

The last is gcc's multiple-include optimization, inferred rather than
declared: entering a file arms it, the first text token outside a conditional
disarms it, any directive but an opening conditional or the null directive
disarms it, an `#else` on the outermost conditional voids the guard, and
reaching end of file with it still armed records the controlling macro. Both
guard spellings are recognized (`#ifndef FOO_H` and `#if !defined FOO_H`).

Marking is per *file*, not per directive: once a file has been `#import`ed, a
later `#include` of it is skipped too, which is what keeps a mixed codebase
from reading `NSObject.h` twice.

`#pragma once` is honoured **silently**. In C it is worth a note — the ISO
spelling is an include guard, and inferring one costs nothing. In Objective-C
it is not: `#import` already says the same thing, the language has no ISO
spelling to prefer, and the platform's own headers use both.

Each file is read and scanned at most once per translation unit whatever
brought it in. The token slice is cached; a second inclusion re-walks it with
a fresh `Origin`, so `__FILE__` and the inclusion chain are right without
re-reading anything.

## Header search

Two lists, in this order:

| | |
| --- | --- |
| `#include "..."` / `#import "..."` | the including file's own directory, then `Search`, then `Frameworks` |
| `#include <...>` / `#import <...>` | `Search`, then `Frameworks` |

A **framework include** is one whose name has a slash in it. `<Foundation/NSString.h>`
is looked for as `Foundation.framework/Headers/NSString.h` in each framework
directory, then as `Foundation.framework/PrivateHeaders/NSString.h`. A header
inside a framework that imports its siblings — which is how every umbrella
header is written — finds them through the same directory the framework was
found in, because that directory is already in the list.

There is no `-iquote`/`-isystem`/`-idirafter` tower, no header maps, and no
module cache. The lists are `Config.Search` and `Config.Frameworks`, computed
once by the `objv` package and printed by `objv env` before the build runs.

**Header names are reconstructed here, not scanned.** `<Foundation/NSString.h>`
is one pp-token only in this context; everywhere else it is `LSS IDENT QUO
IDENT DOT IDENT GTR`. `include.go` recovers it from the raw bytes between `<`
and `>` — bytes, not token spellings, because the slash separating a framework
from its header was never a token. A macro that expands to a header name is
handled: §6.10.2p4's form is tried after the two literal forms fail.

`#include_next` is supported: the same search, resumed after the directory the
including file was found in, which is the only way a header can wrap the one
it shadows. Absolute paths are rejected — an absolute path is the build
machine leaking into the source — and so are paths that escape their mount.

A header that is not found says which list came up empty, because the fix
differs: a framework include with no `-F` is a different mistake from a header
with no `-I`.

## Macro expansion

The implementation is Dave Prosser's X3J11/86-196 pseudocode, function for
function: `expand`, `subst`, `glue`, `hsadd`, with `ts`/`fp`/`select`/
`stringize` as helpers. The standard's §6.10.3.4 prose is a translation of
that memo and loses detail in the translation; the memo is what `expand.go`
implements.

The detail that matters most:

```
subst(ts(T), fp(T), actuals, (HS ∩ HS') ∪ {T}, {})
```

`HS` is the macro name's hide set. `HS'` belongs to the **closing
parenthesis**, not the name. Their *intersection*, plus the macro itself, is
the new hide set. An implementation that unions them, or that ignores `HS'`,
gets DR 017's `NIL` and `a(a)` cases wrong.

Three consequences worth stating, because each is a rule elsewhere and a
structural fact here:

- **A macro is disabled while its expansion is rescanned, and not while its
  arguments are expanded.** The hide set rides on the tokens, so this needs no
  context stack and no enable/disable bookkeeping.
- **Argument pre-expansion cannot reach past the invocation.** An argument is
  a closed sequence — a `stream` with no refill function — so `#define f(x) x`
  invoked as `f(g) (2)` cannot pull the `(2)` in.
- **The hide-set test precedes the search for `(`.** Reversed, finding the
  parenthesis would pop a context and re-enable the macro we are inside.

```
#define foo(x) bar x
foo(foo) (2)          →  bar foo (2)
```

A macro invocation cannot straddle a directive: text expansion stops at a
line-opening `#`, so the argument list runs out of tokens and is reported as
unterminated rather than silently swallowing an `#endif`.

`, ## __VA_ARGS__` is supported. The `##` there does not paste: it marks the
comma as belonging to the variadic argument, so an empty argument takes the
comma with it. Every logging macro in Objective-C is written with it, and
there is no ISO spelling that does the same thing on a C11 substrate.

### `@#x`

`#x` gives an array of characters; `@#x` gives a string object. The two tokens
are one operator in that spelling, so the `@` is taken back off the output and
the result is minted as the single `OBJC_STRING_LIT` §2.4 says an
`ObjectStringLiteral` is:

```objc
#define OBJV_KEY(x) @#x
[d setObject:v forKey:OBJV_KEY(count)];   //  @"count", one token
```

With whitespace between them — `@ #x` — they are two tokens, which is what
they say and what §6.1's `StringLiteralSequence` lets the parser join. The
same is true of a macro that puts an `@` in front of a string argument
(`@fmt`): phase 4 hands the parser `AT` and `STRING_LIT`, and joining them is
the parser's rule, not phase 4's.

### Generated tokens

`stringize` returns a string literal containing concatenated spellings; `glue`
returns a token spelled `L&R`; `_Pragma` returns a `#` and a `pragma`. None of
them exists in any source file, and `token`'s contract says a token is a span
in a position space carrying no text.

So phase 4 supplies a position space. `Gen` is an append-only byte buffer; a
generated token is an ordinary non-empty span in it, reached through an
`Origin` whose `File` is nil. Nothing below phase 4 learns a new concept.

A pasted spelling is **re-scanned** before it becomes a token, because
§6.10.3.3p3 requires the result be a single preprocessing token and only the
scanner can say whether it is. `+` `##` `+` gives `++`; `+` `##` `x` is a
constraint violation, reported once, with both operands left in place so the
rest of the line still parses. Results are cached — macro-heavy headers paste
the same pair thousands of times.

## Spacing

`--emit mi` output must re-enter as `.mi` input and produce a byte-identical
executable. That requires accidental pastes never happen:

```
#define PLUS +
#define EMPTY
+PLUS -EMPTY-        →  + + - -        not  ++ --
```

gcc solves this with padding tokens in the stream. objv does not need them:
the claim is a byte-identical *executable*, not byte-identical text, so
adjacency is carried on the tokens and paste avoidance happens once, at print
time. The stream keeps four tokens either way; what it must never do is lose
the boundary.

What expansion owes the printer is accurate adjacency, and the rules are the
ones spacing actually follows:

- the first token of an expansion inherits the **invocation's** spacing;
- a substituted argument's first token inherits the **parameter's** spacing in
  the replacement list.

```
#define add(x, y, z) x + y +z;
sum = add (1,2, 3)   →  sum = 1 + 2 +3;
```

## `_Pragma`

A macro cannot expand to a directive, so a macro that must open a pragma
region expands to `_Pragma` instead (§6.10.9) and phase 4 turns it back into
one. This is not a corner of the language here:

```objc
#define NS_ASSUME_NONNULL_BEGIN _Pragma("clang assume_nonnull begin")
```

stands at the top of essentially every header in a modern SDK, so a
preprocessor without `_Pragma` cannot read Foundation.

The string is destringized as the standard says — `L` prefix dropped, `\"` and
`\\` unescaped — scanned, and sent down the same path a written `#pragma`
takes, so the two spellings are interchangeable rather than merely similar.
`_Pragma("once")` is `#pragma once`, about the file it appears in. The
operands are stripped of `FlagNLBefore` on the way out: they came from their
own little file, where the first token opened a line, and printing them as a
new line would turn `#pragma clang assume_nonnull begin` into a null directive
followed by a stray identifier.

Everything phase 4 does not act on itself survives into the output for phase 7
to read — `#pragma clang assume_nonnull begin` is a nullability region the
analyzer opens, and it arrives there as tokens.

## `#if`

§6.10.1 is a different language from §6.6, and reusing `analyzer`'s constant
folder would be the wrong semantics rather than a shortcut. There is no
`sizeof`, no enum constant, no cast; arithmetic is in `intmax_t`/`uintmax_t`
regardless of the target's `int`; `defined` is an operator; and every
identifier that survives expansion — including keywords — is `0`.

Order is the standard's, and it is not negotiable:

1. `defined X` and `defined ( X )` are resolved **before** expansion, so
   `#if defined FOO` does not expand `FOO`.
2. The interrogation operators are resolved next, and for the same reason:
   their operands are not expressions.
3. What remains is macro-expanded, and any operator that expansion *produced*
   is resolved then.
4. Every remaining identifier becomes `0`.

A `defined` produced by expansion is undefined behavior (§6.10.1p4) — which is
license to define it. gcc and clang both evaluate it as the operator, real
system headers lean on that, and objv follows under a named warning
(`expansion-defined`) so the nonportability stays visible, once per system
header. An interrogation operator produced by expansion gets **no** warning:
they are not in the standard at all, their meaning is clang's, and clang
evaluates them there — which is what Darwin's `<secure/_string.h>` is written
against.

`&&` and `||` short-circuit, and **the unevaluated side never reports**. That
is decided before the operand is walked rather than unwound afterwards,
because a diagnostic that has been appended has been reported: `#if 0 && 1/0`
is not a division by zero that gets forgiven, it is not a division at all. The
untaken arm of a `?:` is the same. Division by zero in a *taken* branch is one
diagnostic, not a panic.

Inside a skipped group, conditions are **not evaluated at all** — §6.10.1p6
checks skipped groups only for nesting. This is what makes the universal idiom
safe when the header genuinely is not there:

```objc
#if __has_include(<CoreServices/CoreServices.h>)
#import <CoreServices/CoreServices.h>
#endif
```

## The interrogation operators

Nine operators, in two shapes: `__has_include` and `__has_include_next` take a
header name, and `__has_feature`, `__has_extension`, `__has_attribute`,
`__has_builtin` and the three `__is_target_*` take one word. None of the
operands is an expression, so none of them is macro-expanded — a program that
happened to `#define objc_arc 0` is not asking a different question.

They answer **where they stand**, not only in a controlling expression:
`return __has_feature(objc_arc) ? 0 : 1;` is answered here too, because
leaving one standing would hand the parser an identifier no header declares.
An operator with no `(` after it is left alone, which is what `#ifdef
__has_include` tests for.

`__has_include` asks the same `searchList` the directive asks, so the operator
and the directive it guards cannot disagree — but it *stats* rather than
opens, so a header a program asked about and did not import is not a file the
build depends on.

What the other operators answer is not this package's to decide:

| operator | answered by |
| --- | --- |
| `__has_feature` | `Config.Feature` |
| `__has_extension` | `Config.Extension`, falling back to `Feature` |
| `__has_attribute` | `Config.Attribute` |
| `__has_builtin` | `Config.Builtin` (plus the `__is_target_*` operators, which are builtins) |
| `__is_target_arch` and kin | `Config.Triple`, through an alias table (`arm64` is `aarch64`, `macosx` is `macos`) |

A feature is implemented where it is implemented and turned on where the
command line is read; a copy of that rule here would be a second rule that
could disagree with the first. **Nil answers no**, and a header is entitled to
be told no and take its fallback — which is why a zero `Config` reads the SDK
as a 2011 dialect with no generics, no nullability and no fixed-underlying-type
enums. The `objv` package supplies the answers.

This is not an optional nicety. `<Foundation/NSObjCRuntime.h>` decides what
`NS_ENUM` means from `__has_feature(objc_fixed_enum)`;
`NS_DESIGNATED_INITIALIZER` is `__has_attribute(objc_designated_initializer)`
and nothing otherwise; `Availability.h` cannot be read at all without
`__has_include`.

## Predefined macros

`__FILE__`, `__LINE__`, `__DATE__`, `__TIME__` and `__COUNTER__` are computed,
not stored. The rest are plain values:

| | |
| --- | --- |
| `__OBJC__`, `__OBJC2__` | this is Objective-C, on the modern runtime |
| `OBJC_NEW_PROPERTIES` | gcc's name for the same conclusion about `@property` |
| `__STDC__`, `__STDC_VERSION__`, `__STDC_HOSTED__` | the C substrate |
| `__STDC_NO_COMPLEX__`, `__STDC_NO_THREADS__` | §6.10.8.3, true today |
| `__GNUC__` 4, `__GNUC_MINOR__` 2, `__GNUC_PATCHLEVEL__` 1 | what clang reports, and what `<sys/cdefs.h>` requires of a compiler it will speak to |
| `__GNUC_STDC_INLINE__` | says which `inline` this is, because saying `__GNUC__` without it asks for the other one |
| `__OBJV__` | this compiler |
| `IBAction`, `IBOutlet`, `IBInspectable`, `IB_DESIGNABLE`, `IBOutletCollection(_)` | Interface Builder's keywords, which are the compiler's macros and no framework's |

The first seven, plus `defined`, are `Reserved`: the program may not `#define`
or `#undef` them (§6.10.8p2). The compiler installing them is not the program,
which is the one place that check is lifted.

clang spells the Interface Builder macros as attributes it records for a nib
editor to read back out of the AST — `IBAction` as the notorious
`void)__attribute__((ibaction)`. objv emits no such metadata, so it defines
each as what the declaration means with the metadata removed: `IBAction` is
`void`, and the rest are empty. The code compiles the same and means the same.

Every predefine goes through the same `#define` grammar a directive uses, and
so does every `-D`: a body minted by hand is a body whose tokens were never
scanned, and `IBAction` would then reach the parser as an *identifier* spelled
`void` rather than as the keyword. `-D NAME` defines it as `1`; `-D` and `-U`
are applied in command-line order.

Everything **target-dependent** — `__APPLE__`, `__MACH__`, `__CHAR_BIT__`,
`__SIZEOF_LONG__`, `__INT_MAX__` and kin — arrives through
`Config.Predefines` as text. Those are facts about a target model, and this
package does not import one. The `objv` package computes them and puts them in
the config, the same inversion that keeps `sysroot` out of phase 4 and
`__has_feature`'s answer in a hook.

## Determinism

`__DATE__`, `__TIME__`, and anything else time-shaped read `Config.Epoch`.
Nothing in this package reads a clock. `__COUNTER__` counts expansions over
the whole translation unit and resets for no file. `__FILE__` expands to the
path as given — as written on the command line, or as joined from the mount
name and the include spelling. Never absolute.

Diagnostics sort by file, then position, then extent, stably — the contract
`token.SortDiagnostics` holds within one file, extended across the include
graph so two runs interleave identically.

## What is not implemented

| rejected | why |
| --- | --- |
| `#assert`, `#unassert` | removed from GNU C itself; no replacement |
| `#ident`, `#sccs` | no effect on any target objv emits for |
| module maps, `@import` resolution | `@import` is a token phase 4 passes through; modules are the parser's and the analyzer's |

The rule is the language's own (§1.4 of the grammar): an extension is in if it
is in routine use in Objective-C code or in the Cocoa headers, and out
otherwise. `#import`, `#include_next`, `#warning`, `#pragma once`,
`, ## __VA_ARGS__`, `__COUNTER__` and the nine interrogation operators are all
on the near side of that line — not one of them is optional if the goal is to
read a real SDK.

## Dependency output

`Config.TrackDeps` records every file the include graph reached, in first-seen
order. `Deps.Write` renders the one shape build systems consume: the rule,
then a phony target per header so a deleted header does not break the rebuild.

```
main.o: \
  main.m \
  SDK/Frameworks/Foundation.framework/Headers/Foundation.h

SDK/Frameworks/Foundation.framework/Headers/Foundation.h:
```

That is `-MMD -MF -MP` collapsed.

## Files

| | |
| --- | --- |
| `preprocessor.go` | the driver: directive or text, per file |
| `config.go` | mounts, predefines, dialect, the four answer hooks, limits |
| `pptoken.go` | the pp-token view; the generated arena; stringize |
| `directive.go` | the directive line grammar; `#pragma` and `_Pragma` |
| `condition.go` | the `#if` stack, group skipping |
| `eval.go` | the `#if` constant evaluator |
| `macro.go` | the macro table, §6.10.3p2 |
| `expand.go` | hide sets and Prosser's four functions |
| `predefined.go` | `__FILE__`, `__OBJC__`, the epoch clamp |
| `include.go` | the search, frameworks, the open-once cache, guards, deps |
| `operators.go` | `__has_include` and the eight others |
| `position.go` | `Origin`, `Site`, `Expansion` |

## What this package requires of `scanner`

`ScanPP` mode, for four reasons, all of them because a pp-token is not yet a
token:

- **pp-numbers keep their spans unclassified.** `0779` and `1e+` are legal
  pp-numbers and illegal constants, and `#if 0` may legally hide either.
- **Malformed-literal diagnostics defer to phase 7.** They fire for the tokens
  that survive phase 4, and only those.
- **An `@` on an identifier that is not a directive defers with them.** An
  excluded `#if` group may hold `@NSFoo`.
- **A line-opening `#` is returned as `HASH`.** The ordinary mode consumes it
  as trivia and reports once per file, which is correct for `.mi` input and
  wrong here.

Everything routes through one `scanPP` helper.

## Dependencies

Imports [`token`](../token), [`scanner`](../scanner), and `io/fs` — never
`os`, and nothing above it. It is the only package below the CLI that touches
file *contents*, which is what makes the search lists inspectable data rather
than driver folklore.

Imported by `parser`, which consumes its output, and by the `objv` package,
which builds its `Config`.
