# scanner

`package scanner` turns a `*token.File` into a complete token slice.

```
import "github.com/vertex-language/objv/scanner"
```

The whole unit is tokenized up front. Every scan path advances at least one
byte; malformed input yields an exact span and **one** diagnostic, never a
cascade. Nothing is interpreted: literals keep their raw spelling and every
identifier is an `IDENT` — typedef-ness, class names, protocol names, and
the contextual keywords of §2.2 are the parser's business.

## Usage

```go
f := token.NewFile("a.m", src)
toks, diags := scanner.Scan(f, 0)
```

`Scan` is the entire API. The slice always ends in an `EOF` token (the one
zero-width span, positioned at `f.Size()`). Diagnostics are sorted, with
phase 1–2 diagnostics from `token.NewFile` already merged in.

### Mode

| Mode | Effect |
| --- | --- |
| `ScanComments` | Keep `COMMENT` tokens in the stream. |
| `ScanPP` | Scan preprocessing tokens: the input is a `.m`, not a `.mi`. |
| `NoDollarIdents` | Drop `$` from the identifier alphabet (§2.1). |

`ScanComments` keeps `/* */` and `//` in the stream; without it they are
trivia, still reachable via `token.File.Between`. Block comments don't nest;
an unterminated `/*` is one diagnostic and one token to EOF.

`$` in an identifier is an extension that §2.1 turns **on** by default;
`NoDollarIdents` is the command-line option that turns it off.

## The `@` forms

`@` heads three different things, and the scanner tells them apart by what
follows.

| Source | Tokens |
| --- | --- |
| `@interface` | one `AT_INTERFACE` |
| `@ end` | one `AT_END`, with `FlagSpacedAt` |
| `@"hi"` | one `OBJC_STRING_LIT` |
| `@"a" @"b" "c"` | three tokens — §6.1 concatenates, not this package |
| `@42`, `@-1`, `@'c'`, `@(x)`, `@[…]`, `@{…}` | `AT`, then ordinary tokens |
| `@__objc_yes` | `AT` then `BOOL_LIT` — what `@YES` expands to |
| `@NSUTF8StringEncoding` | `AT` then `IDENT`, and one diagnostic |

§2.5's directive list is closed, so the last row is an error rather than a
new directive — and the error says what to write instead, because an `@` on
an identifier is almost always an attempt to box an enumeration constant,
which §6.8 requires be written `@(NSUTF8StringEncoding)`. `@true` and
`@false` get their own message: they are Objective-C++ spellings of `@YES`
and `@NO`.

§2 notes that an implementation will generally tolerate whitespace or a
comment between the `@` and the identifier. objv tolerates the whitespace,
and records it as `FlagSpacedAt`. It does not tolerate a comment, because a
comment between them has nowhere to go: swallowed into the token's span it
is a `COMMENT` token that `ScanComments` promised and did not deliver, and
emitted on its own it is a token sitting inside another token's span. `@
/* why */ interface` is an `AT` and an `IDENT`, and the parser reports the
`@` that heads nothing.

## Maximal munch

Longest token always. The sharp edges are the grammar's:

| Source | Tokens |
| --- | --- |
| `a+++b` | `a` `++` `+` `b` |
| `..` | `.` `.` |
| `...` | `...` |
| `a>>b` | `a` `>>` `b` — never synthesized from two `>`s |
| `@selector(a::)` | `:` `:` — never munched into one |

The last two rows are the two places Objective-C and maximal munch disagree,
and they disagree in opposite directions:

- **`::` is never one token.** §6.4's `SelectorName` is pieces and colons
  with no parameters to name, so a selector whose second piece is nameless
  puts two colons together: `@selector(a::)` names a real method, the one
  declared `- (void)a:(int)x :(int)y`. The scoped `AttributeName` of §5.9
  that `::` otherwise serves is reached only from the bracketed `[[…]]`
  attribute form, where the parser recognizes the pair from `FlagAdjacent`.
- **`>>` is always one token.** `NSArray<id<NSCopying>>` closes two of
  §5.5's angle-bracket lists at once, but nothing below the parser knows a
  list is open. The parser does, and calls `token.Token.SplitAngle` to take
  the token apart with exact spans when it needs the first `>` alone. The
  same applies to `>>=` and `>=`.

**Literal prefixes munch too**: `u`, `U`, `L` before a quote are part of the
literal — `u8"…"`, `u"…"`, `U"…"`, `L"…"`, `u'…'`, `U'…'`, `L'…'` each scan
as one token. A `u`, `U`, or `L` *not* followed by a quote is an ordinary
identifier. §2.4 forbids an `EncodingPrefix` on an `ObjectStringLiteral`, so
`u8@"x"` is an identifier and then an `OBJC_STRING_LIT`.

## Digraphs and directive lines

Digraphs scan as their canonical kinds with `FlagDigraph` set — so `a<:2:>`
and `a[2]` produce identical token kinds — and the advisory bracket stack
treats them as their canonical bracket. Trigraphs never reach the scanner
(`token.NewFile` replaced them in phase 1).

Without `ScanPP` the scanner is above phase 4: a `#` (or `%:`) opening a
logical line is consumed to end of line as trivia and reported **once per
file**. A line marker (`# 42 "foo.m" 3`, or `#line 42 "foo.m"`) is not
reported — it is what `objv build --emit mi` writes, and `.mi` input is
exactly that output. `#` anywhere else scans as `HASH` for the parser to
reject.

## Identifiers

Keyword lookup is by exact spelling (`token.Lookup`); everything else is
`IDENT`. Universal character names (`\uXXXX`, `\UXXXXXXXX`) inside an
identifier stay undecoded — the span keeps the literal `\u`/`\U` spelling —
and a malformed one (too few hex digits) is one diagnostic, scanning
continuing past it.

§2.1's alphabet reaches an extended character only through a UCN, so one
written directly is not an identifier character: a run of them is one
`ILLEGAL` token and one diagnostic naming the character, not one per byte.
Inside a comment, a string, or a character constant those bytes never reach
this path and are nobody's business. A bare `\` not introducing a UCN is
illegal too: one diagnostic (`stray '\' in program`) and an `ILLEGAL` token,
not folded into the surrounding identifier.

## Literals stay undecoded

`0x1Fu` stays five bytes; string and character literals keep prefixes,
quotes, and any `@`. Adjacent string literals are **not** concatenated —
that is §6.1's `StringLiteralSequence`, above this package, and it is where
an `@` on one piece reaches the others.

Numeric scanning enforces the lexical grammar of §2.3, one diagnostic per
run:

- `0779` — one malformed octal constant, not two numbers.
- `0779.5` is a decimal float, not an octal error — the `.` changes the
  whole run's classification.
- `0x1.8` without a `p` exponent is reported; `0x1.8p3` and `0x1p+4f` are
  clean.
- `0b1011` is C23's binary constant and gcc's extension before it; there is
  no floating form.
- Suffixes are validated: `ul`, `llu`, `ULL` pass; `lul`, `lL` are one
  diagnostic each.
- C11 has no digit separators: `1_024` is `1` then identifier `_024`.
- A run with **two or more dots** is a version number, not a constant:
  `10.12.1` is a legal pp-number that no phase gives a value to, so it
  scans clean. §6.10's `VersionTuple` and the availability attributes on
  nearly every Cocoa declaration are written exactly this way, and the
  parser reads the digits back out of the span.

`''` is reported (at least one `CChar` required); the multi-character `'ab'`
scans clean — its value is a decoding concern. Escape sequences are
recognized but not decoded; `\x` requires at least one hex digit, unknown
escapes (`\q`) are one diagnostic, and `\u`/`\U` inside a literal reuse the
same UCN scanning as identifiers.

Because `token.NewFile` splices lines in phase 2, a raw line terminator
inside a literal is exactly what it looks like: unterminated, reported once,
token still emitted.

## `ScanPP`: below phase 4

`ScanPP` says the input is a source file rather than the output of a
preprocessor. Four things change, all because the package is now running
below phase 4 instead of above it:

1. A line-opening `#` is a `HASH` token, not trivia, and the once-per-file
   report is off. A directive in a `.m` file is not a mistake.
2. The bracket stack is off. `#define BEGIN {` is a real header idiom;
   bracket balance is a claim about preprocessed source.
3. The first token carries `FlagNLBefore`, because it does open a logical
   line — which is what makes a `#` in column 1 of line 1 a directive.
4. Value-level diagnostics defer. A pp-number is not yet a constant, and an
   excluded `#if` group or an unexpanded macro body may hold `0779`, `'\q'`,
   or `@NSFoo` without it being anyone's mistake. Classification still
   happens — phase 4 needs `INT_LIT` vs `FLOAT_LIT`, and `@interface` is an
   `AT_INTERFACE` in both modes — but those reports fire only for the tokens
   that survive: the `#if` evaluator reports the ones it decodes, and
   everything reaching phase 5 is scanned again without `ScanPP`.

Token-formation errors — unterminated literals, malformed UCNs, stray `\` —
are about whether a pp-token exists rather than what it is worth, and report
in every mode.

## Diagnostics and recovery

Every reported span is non-empty and clamped to the source. An advisory
bracket stack (never affecting tokenization) reports:

- a closer with nothing open → `unmatched )`, once;
- a mismatch → blame the *opener* (`unclosed {, closed by )`), then go quiet;
- EOF with openers left → the innermost one.

After the first report the stack stops talking. One brace typo produces one
message.
