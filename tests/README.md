# tests

Corpora, each named for the question it asks. A file belongs in exactly one
of them.

## syntax/

Does it parse? 24 files covering the grammar, from identifiers to the
combinations that only break a parser when they meet. Nothing here has to
mean anything or run — several files declare classes with no superclass and
send messages to nothing — so the only question asked of them is whether the
parser accepts what clang accepts.

Each file is named for the section of [`docs/objc_grammar.md`](../docs/objc_grammar.md)
it covers and cites the productions in comments, so a failure names the
construct rather than the file. The numbering runs from the lexical
structure outward: the C substrate first (01–08), then Objective-C's own
declarations (09–17), then its expressions and statements (18–23), then
`24-cocoa-idioms.m`, which introduces nothing and only puts the rest
together the way a framework header does.

Used by `parser`, in `parse_test.go`.

### On clang as the oracle

The corpus is checked against clang, which is the only answer to "is this
really Objective-C" that cannot be argued with:

```console
$ xcrun clang -fsyntax-only -Wno-everything tests/syntax/*.m
```

Every file passes clang's **parser**. What clang still reports is semantic,
and is the price of a corpus with no headers in it: `nil` is a macro from
`<objc/objc.h>`, `__weak` needs ARC, an array literal needs a real
`NSArray`, and `@import` needs `-fmodules`. A syntax error from clang is a
bug in the corpus or in this compiler; a semantic one is neither, and the
corpus is deliberately hermetic so that a parser test never depends on which
SDK is installed.

That oracle has already earned its keep. `- (void)a::(int)x;` was in this
corpus, and in a comment in `token/kind.go`, as the reason the scanner must
not munch `::`. clang rejects it — §4.7 gives every `KeywordDeclarator` an
`Identifier` after its type — and the real case is `@selector(a::)`, where a
`SelectorName` has no parameters to name.

## check/

Does it typecheck, and does it say the right thing when it does not?

A file's name is its contract. `ok-*` must produce no errors. `bad-*` must
produce the errors it names — and it names each one **where it happens**:

```objc
[self pnig];              // expect: no instance method 'pnig' on 'Target'
```

A line carrying `// expect: <text>` must draw a diagnostic whose message
contains that text, and a diagnostic on a line that asked for none fails the
test. That is clang's `-verify` in miniature, and it is worth the machinery: a
test that only counted diagnostics would pass when the compiler reported the
right number of the wrong things. A line may carry several markers, since one
mistake often breaks more than one thing.

`prelude.txt` is prepended to every file. The corpus is hermetic — no headers,
no SDK — so the handful of classes the *language* names have to be declared
somewhere: §6.8's literals are sends to NSString, NSNumber, NSArray and
NSDictionary, and every class needs a root to inherit from.

Used by `analyzer`, in `corpus_test.go`. The syntax corpus is run through the
analyzer too, as a crash test.

## eval/

Does it become the right IR?

A file states what it expects as substrings of the emitted module:

```objc
- (int)count { return _count; }
// vir: ptr.getaddr @_OBJC_IVAR_$_Counter$_count
// vir-not: memcpy
```

Substrings and not line numbers, because a lowering has no line to be on: one
statement becomes several blocks and one expression becomes several
instructions, and a test anchored to a line would fail on every change to the
order things are emitted in rather than on a change to what is emitted. What a
marker names is a fact about the module — this symbol exists, this section is
used, this instruction is reached — which is what a reader of the file wanted
to know anyway.

Every file must also lower with no diagnostics and pass `verify.Module` from
the `ir` package. That is the half of the contract no marker states, and it is
the half that catches the bugs: an initializer whose shape does not match its
declared type prints perfectly well.

`prelude.txt` is `check/`'s with the class methods §6.8's literals send. The
analyzer only has to know that NSNumber exists; lowering emits a real
`numberWithInt:` to it, and a selector nothing declared is not lowered.

Used by `lower`, in `corpus_test.go`. The syntax corpus is run through
lowering too, as a crash test.

## programs/

Does the program do what it says?

Whole Objective-C programs against the real SDK, built twice — once by objv,
once by clang — run, and compared. Nothing here writes down an expected value:
a number beside a program is a claim that has to be maintained by hand and is
wrong the moment the program drifts. clang's own output is a claim that
maintains itself, and a disagreement with it is a bug in this compiler by
definition.

The files are meant to look like code someone would write — a word counter, a
settings store, a little event bus — and to be dense in the constructs that
meet each other only in real programs: a block captured by a collection, a
category on a framework class, a property whose setter copies, a protocol
dispatched through `id`, an exception crossing three frames. That is the
point. Every other corpus tests a layer against what that layer is supposed to
do; this one tests the compiler against a program, which is the only thing
that finds what no layer's author thought to ask about.

Each file is built in both memory models unless it says otherwise:

```objc
// mode: arc          // or mrr; the default is both
// frameworks: AppKit // Foundation is always linked
```

Deterministic output only, on stdout: no clock, no addresses, no hash order,
no concurrency. A dictionary's keys are sorted before they are printed, which
is a thing the program has to do anyway.

Used by `objv`, in `programs_test.go`. It is skipped off arm64 macOS and
skipped without clang, because both halves of the comparison have to run.

## What is not here yet

| | |
| --- | --- |
| `interop/` | Is what objv builds the same thing clang builds? A category compiled by clang, a class compiled by objv, one process. The metadata the runtime walks has to be the metadata clang emitted. |
