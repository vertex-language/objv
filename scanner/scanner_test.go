package scanner

import (
	"testing"

	"github.com/vertex-language/objv/token"
)

func scan(t *testing.T, src string, mode Mode) ([]token.Token, []token.Diagnostic) {
	t.Helper()
	return Scan(token.NewFile("a.m", []byte(src+"\n")), mode)
}

func wantKinds(t *testing.T, src string, want ...token.Kind) []token.Diagnostic {
	t.Helper()
	toks, diags := scan(t, src, 0)
	want = append(want, token.EOF)
	if len(toks) != len(want) {
		t.Fatalf("%q: got %d tokens %v, want %d", src, len(toks), toks, len(want))
	}
	for i, k := range want {
		if toks[i].Kind != k {
			t.Fatalf("%q: token %d = %v, want %v", src, i, toks[i].Kind, k)
		}
	}
	return diags
}

func errCount(ds []token.Diagnostic) int {
	n := 0
	for _, d := range ds {
		if d.Severity == token.Error {
			n++
		}
	}
	return n
}

func warnCount(ds []token.Diagnostic) int {
	n := 0
	for _, d := range ds {
		if d.Severity == token.Warn {
			n++
		}
	}
	return n
}

func TestMaximalMunch(t *testing.T) {
	wantKinds(t, "a+++b", token.IDENT, token.INC, token.ADD, token.IDENT)
	wantKinds(t, "a+++++b", token.IDENT, token.INC, token.INC, token.ADD, token.IDENT)
	wantKinds(t, "..", token.PERIOD, token.PERIOD)
	wantKinds(t, "...", token.ELLIPSIS)
	wantKinds(t, "a<<=b", token.IDENT, token.SHL_ASSIGN, token.IDENT)
	wantKinds(t, "a>>b", token.IDENT, token.SHR, token.IDENT)
}

func TestKeywordsAndIdents(t *testing.T) {
	wantKinds(t, "typedef T _Bool _bool",
		token.TYPEDEF, token.IDENT, token.BOOL, token.IDENT)

	// The Objective-C keywords, and the typedef names that are not
	// keywords however much they look like them.
	wantKinds(t, "__strong id obj = (__bridge id)p;",
		token.STRONG, token.IDENT, token.IDENT, token.ASSIGN,
		token.LPAREN, token.BRIDGE, token.IDENT, token.RPAREN, token.IDENT, token.SEMI)

	// instancetype and the property attribute names are contextual:
	// identifiers everywhere the parser is not looking for them.
	wantKinds(t, "instancetype nonatomic copy nonnull",
		token.IDENT, token.IDENT, token.IDENT, token.IDENT)

	// Alias spellings arrive as the kind they spell.
	wantKinds(t, "__typeof__(x) __nonnull __thread",
		token.TYPEOF, token.LPAREN, token.IDENT, token.RPAREN,
		token.NONNULL, token.THREAD_LOCAL)
}

func TestDollarIdentifiers(t *testing.T) {
	// §2.1: on by default, and off under the option.
	wantKinds(t, "a$b $x", token.IDENT, token.IDENT)
	toks, diags := scan(t, "a$b", NoDollarIdents)
	if len(toks) != 4 || toks[0].Kind != token.IDENT || toks[1].Kind != token.ILLEGAL {
		t.Fatalf("under NoDollarIdents: %v", toks)
	}
	if errCount(diags) != 1 {
		t.Errorf("under NoDollarIdents: %v, want one diagnostic on the $", diags)
	}
}

// ---- §2.5 directives ----

func TestDirectives(t *testing.T) {
	toks, diags := scan(t, "@interface Foo : NSObject\n@end", 0)
	want := []token.Kind{token.AT_INTERFACE, token.IDENT, token.COLON,
		token.IDENT, token.AT_END, token.EOF}
	for i, k := range want {
		if toks[i].Kind != k {
			t.Fatalf("token %d = %v, want %v", i, toks[i].Kind, k)
		}
	}
	if errCount(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
	// One token, spanning the @ and the keyword.
	f := token.NewFile("a.m", []byte("@autoreleasepool {}\n"))
	toks, _ = Scan(f, 0)
	if got := string(f.Slice(toks[0].Pos, toks[0].End)); got != "@autoreleasepool" {
		t.Errorf("directive span = %q, want the @ and the keyword together", got)
	}
}

func TestEveryDirectiveScans(t *testing.T) {
	for _, name := range []string{
		"interface", "implementation", "protocol", "end", "class",
		"compatibility_alias", "import", "property", "synthesize", "dynamic",
		"required", "optional", "private", "protected", "public", "package",
		"selector", "encode", "defs", "available", "try", "catch", "finally",
		"throw", "synchronized", "autoreleasepool",
	} {
		src := "@" + name
		toks, diags := scan(t, src, 0)
		if len(toks) != 2 || !toks[0].Kind.IsDirective() || toks[0].Kind.String() != src {
			t.Errorf("%q scanned as %v", src, toks)
		}
		if errCount(diags) != 0 {
			t.Errorf("%q: %v", src, diags)
		}
	}
}

func TestSpacedAt(t *testing.T) {
	// §2 tolerates whitespace between the @ and its keyword.
	toks, diags := scan(t, "@ end", 0)
	if toks[0].Kind != token.AT_END || !toks[0].Flags.Has(token.FlagSpacedAt) {
		t.Fatalf("`@ end` = %v flags %v, want AT_END with FlagSpacedAt", toks[0].Kind, toks[0].Flags)
	}
	if errCount(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
	if toks, _ := scan(t, "@end", 0); toks[0].Flags.Has(token.FlagSpacedAt) {
		t.Error("`@end` should not carry FlagSpacedAt")
	}
	// A comment is not tolerated: it has nowhere to go inside a token.
	wantKinds(t, "@ /*x*/ end", token.AT, token.IDENT)
}

func TestUnknownDirective(t *testing.T) {
	// §2.5's list is closed, which is also why §6.8 cannot box an
	// enumeration constant by name.
	toks, diags := scan(t, "id x = @NSUTF8StringEncoding;", 0)
	if toks[3].Kind != token.AT || toks[4].Kind != token.IDENT {
		t.Fatalf("tokens = %v, want the @ to stand alone", toks)
	}
	if errCount(diags) != 1 {
		t.Fatalf("diagnostics = %v, want exactly one", diags)
	}
	if want := "@NSUTF8StringEncoding is not a directive; to box a constant write @(NSUTF8StringEncoding)"; diags[0].Message != want {
		t.Errorf("message = %q, want %q", diags[0].Message, want)
	}

	// @true and @false are Objective-C++ spellings and get their own.
	_, diags = scan(t, "id b = @true;", 0)
	if errCount(diags) != 1 || diags[0].Message != "@true is Objective-C++ only; write @YES in Objective-C" {
		t.Errorf("@true: %v", diags)
	}
}

func TestBoxedExpressions(t *testing.T) {
	// §6.8: each of these is an @ and then an ordinary token.
	wantKinds(t, "@42", token.AT, token.INT_LIT)
	wantKinds(t, "@-1", token.AT, token.SUB, token.INT_LIT)
	wantKinds(t, "@3.5f", token.AT, token.FLOAT_LIT)
	wantKinds(t, "@'c'", token.AT, token.CHAR_LIT)
	wantKinds(t, "@(x + 1)", token.AT, token.LPAREN, token.IDENT, token.ADD,
		token.INT_LIT, token.RPAREN)
	wantKinds(t, "@[a, b]", token.AT, token.LBRACK, token.IDENT, token.COMMA,
		token.IDENT, token.RBRACK)
	wantKinds(t, "@{k: v}", token.AT, token.LBRACE, token.IDENT, token.COLON,
		token.IDENT, token.RBRACE)

	// YES and NO are macros; what reaches the scanner is what they
	// expand to, and §2.3 makes it a constant rather than an identifier
	// so that a boxed boolean is not a boxed integer.
	if d := wantKinds(t, "@__objc_yes", token.AT, token.BOOL_LIT); errCount(d) != 0 {
		t.Errorf("@__objc_yes: %v, want no diagnostic", d)
	}
	wantKinds(t, "__objc_no", token.BOOL_LIT)
}

func TestObjectStringLiteral(t *testing.T) {
	f := token.NewFile("a.m", []byte("@\"hi\"\n"))
	toks, diags := Scan(f, 0)
	if toks[0].Kind != token.OBJC_STRING_LIT {
		t.Fatalf("tokens = %v", toks)
	}
	if got := string(f.Slice(toks[0].Pos, toks[0].End)); got != `@"hi"` {
		t.Errorf("span = %q, want the @ and the literal as one token", got)
	}
	if errCount(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}

	// §6.1 concatenates a sequence, but that is above this package:
	// each piece is its own token, with or without an @.
	wantKinds(t, `@"a" @"b" "c"`, token.OBJC_STRING_LIT, token.OBJC_STRING_LIT, token.STRING_LIT)

	// An encoding prefix may not be applied to one (§2.4): u8 is an
	// identifier here, and the @"…" that follows is its own token.
	wantKinds(t, `u8@"x"`, token.IDENT, token.OBJC_STRING_LIT)

	// Unterminated: one report, token still emitted.
	toks, diags = scan(t, "@\"abc\nx", 0)
	if toks[0].Kind != token.OBJC_STRING_LIT || toks[1].Kind != token.IDENT {
		t.Fatalf("tokens = %v", toks)
	}
	if errCount(diags) != 1 {
		t.Errorf("diagnostics = %v, want exactly one", diags)
	}
}

// ---- the Objective-C shapes that maximal munch could get wrong ----

func TestAdjacentColonsAreNotOneToken(t *testing.T) {
	// §6.4's SelectorName has no parameters to name, so a selector whose
	// second piece is nameless puts two colons together: @selector(a::)
	// names the method declared `- (void)a:(int)x :(int)y`. Munching ::
	// would lose it. The scoped AttributeName of §5.9 that :: otherwise
	// serves is recovered from FlagAdjacent instead.
	wantKinds(t, "- (void)a:(int)x :(int)y;",
		token.SUB, token.LPAREN, token.VOID, token.RPAREN, token.IDENT,
		token.COLON, token.LPAREN, token.INT, token.RPAREN, token.IDENT,
		token.COLON, token.LPAREN, token.INT, token.RPAREN, token.IDENT,
		token.SEMI)

	toks, _ := scan(t, "@selector(a::)", 0)
	if toks[3].Kind != token.COLON || toks[4].Kind != token.COLON {
		t.Fatalf("tokens = %v", toks)
	}
	if !toks[4].Flags.Has(token.FlagAdjacent) {
		t.Error("the second colon is adjacent to the first")
	}
	toks, _ = scan(t, "[[clang::objc_arc]]", 0)
	if toks[4].Kind != token.COLON || !toks[4].Flags.Has(token.FlagAdjacent) {
		t.Error("a scoped attribute name's :: is two adjacent colons")
	}
}

func TestNestedGenericsCloseWithSHR(t *testing.T) {
	// NSArray<id<NSCopying>> closes two of §5.5's lists at once.
	// Nothing below the parser knows a list is open, so the scanner
	// munches, and token.Token.SplitAngle undoes it where the parser
	// needs the first > alone.
	f := token.NewFile("a.m", []byte("NSArray<id<NSCopying>> *a;\n"))
	toks, diags := Scan(f, 0)
	var shr token.Token
	for _, tk := range toks {
		if tk.Kind == token.SHR {
			shr = tk
		}
	}
	if shr.Kind != token.SHR {
		t.Fatalf("tokens = %v, want an SHR closing both lists", toks)
	}
	lead, rest, ok := shr.SplitAngle()
	if !ok || lead.Kind != token.GTR || rest.Kind != token.GTR {
		t.Fatalf("SplitAngle = %v %v %v", lead, rest, ok)
	}
	if got := string(f.Slice(lead.Pos, lead.End)) + string(f.Slice(rest.Pos, rest.End)); got != ">>" {
		t.Errorf("split spans cover %q, want \">>\"", got)
	}
	if errCount(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestMessageSendAndBlocks(t *testing.T) {
	wantKinds(t, "[[Foo alloc] initWithName:@\"x\" count:2]",
		token.LBRACK, token.LBRACK, token.IDENT, token.IDENT, token.RBRACK,
		token.IDENT, token.COLON, token.OBJC_STRING_LIT, token.IDENT,
		token.COLON, token.INT_LIT, token.RBRACK)

	// ^ is the block introducer and the exclusive-OR operator both.
	wantKinds(t, "void (^b)(int) = ^(int x){ return x ^ 1; };",
		token.VOID, token.LPAREN, token.XOR, token.IDENT, token.RPAREN,
		token.LPAREN, token.INT, token.RPAREN, token.ASSIGN, token.XOR,
		token.LPAREN, token.INT, token.IDENT, token.RPAREN, token.LBRACE,
		token.RETURN, token.IDENT, token.XOR, token.INT_LIT, token.SEMI,
		token.RBRACE, token.SEMI)
}

func TestVersionTuples(t *testing.T) {
	// §6.10's VersionTuple and the availability attributes on nearly
	// every Cocoa declaration are pp-numbers with two dots: legal, and
	// given a value by no phase, so there is nothing to report.
	toks, diags := scan(t, "if (@available(macOS 10.12.1, *)) {}", 0)
	if toks[2].Kind != token.AT_AVAILABLE {
		t.Fatalf("tokens = %v", toks)
	}
	if errCount(diags) != 0 {
		t.Errorf("%v, want none", diags)
	}
	wantKinds(t, "__builtin_available(iOS 13.0, *)",
		token.BUILTIN_AVAILABLE, token.LPAREN, token.IDENT, token.FLOAT_LIT,
		token.COMMA, token.MUL, token.RPAREN)
}

// ---- the C substrate ----

func TestDigraphs(t *testing.T) {
	toks, diags := scan(t, "<: :> <% %> a %: b", 0)
	want := []token.Kind{token.LBRACK, token.RBRACK, token.LBRACE, token.RBRACE,
		token.IDENT, token.HASH, token.IDENT, token.EOF}
	for i, k := range want {
		if toks[i].Kind != k {
			t.Fatalf("token %d = %v, want %v", i, toks[i].Kind, k)
		}
	}
	for _, i := range []int{0, 1, 2, 3, 5} {
		if !toks[i].Flags.Has(token.FlagDigraph) {
			t.Errorf("token %d (%v): FlagDigraph not set", i, toks[i].Kind)
		}
	}
	if len(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestNumbers(t *testing.T) {
	cases := []struct {
		src   string
		kind  token.Kind
		diags int
	}{
		{"0x1Fu", token.INT_LIT, 0},
		{"0b1011", token.INT_LIT, 0},
		{"0b", token.INT_LIT, 1},
		{"0779", token.INT_LIT, 1},
		{"0779.5", token.FLOAT_LIT, 0}, // decimal float, not octal
		{"0x1.8", token.FLOAT_LIT, 1},  // missing binary exponent
		{"0x1.8p3", token.FLOAT_LIT, 0},
		{"0x1p+4f", token.FLOAT_LIT, 0},
		{"1e+5", token.FLOAT_LIT, 0},
		{".5", token.FLOAT_LIT, 0},
		{"1e", token.FLOAT_LIT, 1},
		{"1ul", token.INT_LIT, 0},
		{"2llu", token.INT_LIT, 0},
		{"3ULL", token.INT_LIT, 0},
		{"4lul", token.INT_LIT, 1},
		{"5lL", token.INT_LIT, 1},
		{"10.12.1", token.FLOAT_LIT, 0}, // a version tuple
	}
	for _, c := range cases {
		toks, diags := scan(t, c.src, 0)
		if toks[0].Kind != c.kind {
			t.Errorf("%q: kind = %v, want %v", c.src, toks[0].Kind, c.kind)
		}
		if len(toks) != 2 { // literal + EOF: one run, one token
			t.Errorf("%q: %d tokens, want 2", c.src, len(toks))
		}
		if n := errCount(diags); n != c.diags {
			t.Errorf("%q: %d error diagnostics %v, want %d", c.src, n, diags, c.diags)
		}
	}
}

func TestLiteralsStayUndecoded(t *testing.T) {
	f := token.NewFile("a.m", []byte("0x1Fu\n"))
	toks, _ := Scan(f, 0)
	if got := string(f.Slice(toks[0].Pos, toks[0].End)); got != "0x1Fu" {
		t.Errorf("literal span = %q, want the five raw bytes", got)
	}
}

func TestDigitSeparatorIsNotC11(t *testing.T) {
	diags := wantKinds(t, "1_024", token.INT_LIT, token.IDENT)
	if errCount(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestCharConstants(t *testing.T) {
	if d := wantKinds(t, "''", token.CHAR_LIT); errCount(d) != 1 {
		t.Errorf("'': %v, want one diagnostic", d)
	}
	if d := wantKinds(t, "'ab'", token.CHAR_LIT); errCount(d) != 0 {
		t.Errorf("'ab' should scan clean: %v", d)
	}
	if d := wantKinds(t, `'\q'`, token.CHAR_LIT); errCount(d) != 1 {
		t.Errorf(`'\q': %v, want one diagnostic`, d)
	}
	if d := wantKinds(t, `L'a'`, token.CHAR_LIT); errCount(d) != 0 {
		t.Errorf("L'a': %v", d)
	}
}

func TestStrings(t *testing.T) {
	// Adjacent literals are NOT concatenated: §6.1 is above us.
	wantKinds(t, `"a" "b"`, token.STRING_LIT, token.STRING_LIT)
	wantKinds(t, `u8"x"`, token.STRING_LIT)

	// A raw newline ends the literal: one report, token still emitted.
	toks, diags := scan(t, "\"abc\nx", 0)
	if toks[0].Kind != token.STRING_LIT || toks[1].Kind != token.IDENT {
		t.Fatalf("tokens = %v", toks)
	}
	if errCount(diags) != 1 {
		t.Errorf("diagnostics = %v, want exactly one", diags)
	}
}

func TestIdentifierUCN(t *testing.T) {
	if d := wantKinds(t, `\u00C5ngstr\u00F6m`, token.IDENT); errCount(d) != 0 {
		t.Errorf("universal character names in an identifier: %v", d)
	}
	// Written directly, an extended character is not in §2.1's
	// alphabet: one ILLEGAL token per run, not one per byte.
	d := wantKinds(t, `Ångström`, token.ILLEGAL, token.IDENT, token.ILLEGAL, token.IDENT)
	if errCount(d) != 2 {
		t.Errorf("raw UTF-8 in an identifier: %v, want one diagnostic per character", d)
	}
	if d := wantKinds(t, `/* Ångström */ x`, token.IDENT); errCount(d) != 0 {
		t.Errorf("extended characters in a comment are nobody's business: %v", d)
	}
	if d := wantKinds(t, `@"Ångström"`, token.OBJC_STRING_LIT); errCount(d) != 0 {
		t.Errorf("extended characters in a literal are nobody's business: %v", d)
	}
	if d := wantKinds(t, `\u00C`, token.IDENT); errCount(d) != 1 {
		t.Errorf("malformed UCN: %v, want one diagnostic", d)
	}
	if d := wantKinds(t, `a\t`, token.IDENT, token.ILLEGAL, token.IDENT); errCount(d) != 1 {
		t.Errorf(`stray backslash: %v, want one diagnostic`, d)
	}
}

func TestDirectiveLinesAreTrivia(t *testing.T) {
	toks, diags := scan(t, "# 1 \"a.m\"\nint x;\n# 2 \"a.m\"\n%: 3", 0)
	wantK := []token.Kind{token.INT, token.IDENT, token.SEMI, token.EOF}
	if len(toks) != len(wantK) {
		t.Fatalf("tokens = %v", toks)
	}
	for i, k := range wantK {
		if toks[i].Kind != k {
			t.Fatalf("token %d = %v, want %v", i, toks[i].Kind, k)
		}
	}
	if warnCount(diags) != 1 {
		t.Errorf("want the directive report once per file, got %v", diags)
	}
}

func TestBracketStack(t *testing.T) {
	for _, c := range []struct {
		src  string
		want int
	}{
		{"a)", 1},     // unmatched closer, once
		{"(]", 1},     // mismatch blames the opener, then quiet
		{"(] ) }", 1}, // ... and stays quiet
		{"((", 1},     // EOF: the innermost opener
		{"({[]})", 0},
		{"<: :>", 0}, // digraphs participate as their canonical kinds
		{"[[Foo alloc] init]", 0},
		{"@{@\"k\": @[@1]}", 0}, // collection literals nest like any bracket
	} {
		_, diags := scan(t, c.src, 0)
		if n := errCount(diags); n != c.want {
			t.Errorf("%q: %d error diagnostics %v, want %d", c.src, n, diags, c.want)
		}
	}
}

func TestFlags(t *testing.T) {
	toks, _ := scan(t, "a+ b\nc", 0)
	// a + b c EOF
	if !toks[1].Flags.Has(token.FlagAdjacent) {
		t.Error("'+' should be FlagAdjacent to 'a'")
	}
	if toks[2].Flags.Has(token.FlagAdjacent) {
		t.Error("'b' is separated by a space")
	}
	if !toks[3].Flags.Has(token.FlagNLBefore) {
		t.Error("'c' should have FlagNLBefore")
	}
	if toks[1].Flags.Has(token.FlagNLBefore) {
		t.Error("'+' should not have FlagNLBefore")
	}
}

func TestComments(t *testing.T) {
	toks, _ := scan(t, "/*x*/a//y", ScanComments)
	want := []token.Kind{token.COMMENT, token.IDENT, token.COMMENT, token.EOF}
	for i, k := range want {
		if toks[i].Kind != k {
			t.Fatalf("token %d = %v, want %v", i, toks[i].Kind, k)
		}
	}
	toks, _ = scan(t, "/*x*/a//y", 0)
	if len(toks) != 2 || toks[0].Kind != token.IDENT {
		t.Fatalf("without ScanComments: %v", toks)
	}
	// Unterminated block comment: one diagnostic, runs to EOF.
	_, diags := scan(t, "/* open", 0)
	if errCount(diags) != 1 {
		t.Errorf("unterminated /*: %v", diags)
	}
}

func TestEOFToken(t *testing.T) {
	f := token.NewFile("a.m", []byte("x;\n"))
	toks, _ := Scan(f, 0)
	last := toks[len(toks)-1]
	if last.Kind != token.EOF || last.Pos != last.End || last.Pos != f.Pos(f.Size()) {
		t.Errorf("EOF token = %+v, want zero-width at Size()", last)
	}
}

func TestEmptyFile(t *testing.T) {
	toks, diags := Scan(token.NewFile("a.m", nil), 0)
	if len(toks) != 1 || toks[0].Kind != token.EOF {
		t.Errorf("tokens = %v, want just EOF", toks)
	}
	if len(diags) != 0 {
		t.Errorf("an empty file has nothing to report: %v", diags)
	}
}

func TestNoFinalNewlineScansIdentically(t *testing.T) {
	// EOF terminates the final line: same tokens, same flags, same
	// diagnostics, with or without the trailing newline.
	for _, c := range []struct {
		src  string
		mode Mode
	}{
		{"int x;", 0},
		{"int x; // trailing", 0},
		{"int x; /* open", 0}, // unterminated /*: same one report both ways
		{"@end", 0},
		{"@", 0},
		{"#import <Foundation/Foundation.h>", ScanPP},
		{"#if 1", ScanPP},
	} {
		a, ad := Scan(token.NewFile("a.m", []byte(c.src)), c.mode)
		b, bd := Scan(token.NewFile("a.m", []byte(c.src+"\n")), c.mode)
		if len(a) != len(b) {
			t.Fatalf("%q (mode %d): %d tokens without newline, %d with",
				c.src, c.mode, len(a), len(b))
		}
		for i := range a {
			if a[i].Kind != b[i].Kind {
				t.Errorf("%q (mode %d): token %d = %v without newline, %v with",
					c.src, c.mode, i, a[i].Kind, b[i].Kind)
			}
			if a[i].Kind != token.EOF && a[i].Flags != b[i].Flags {
				t.Errorf("%q (mode %d): token %d flags = %v without newline, %v with",
					c.src, c.mode, i, a[i].Flags, b[i].Flags)
			}
		}
		if errCount(ad) != errCount(bd) || warnCount(ad) != warnCount(bd) {
			t.Errorf("%q (mode %d): diagnostics differ: %v without newline, %v with",
				c.src, c.mode, ad, bd)
		}
	}
}

// ---- ScanPP: scanning source rather than preprocessed source ----

func TestScanPPDirectiveIsAToken(t *testing.T) {
	toks, diags := scan(t, "#import <Foundation/Foundation.h>\nid x;", ScanPP)

	if toks[0].Kind != token.HASH {
		t.Fatalf("token 0 = %v, want HASH", toks[0].Kind)
	}
	// The '#' opens a logical line even though nothing precedes it.
	// This is what nlBefore's ScanPP seed is for, and the
	// preprocessor's directive-vs-punctuator test rests on it.
	if !toks[0].Flags.Has(token.FlagNLBefore) {
		t.Error("a '#' in column 1 of line 1 opens a logical line")
	}
	if toks[1].Kind != token.IDENT {
		t.Errorf("token 1 = %v, want the identifier 'import'", toks[1].Kind)
	}
	if !toks[1].Flags.Has(token.FlagAdjacent) {
		t.Error("'import' is adjacent to '#'")
	}
	if warnCount(diags) != 0 {
		t.Errorf("a directive in .m input is not a mistake: %v", diags)
	}

	// Without ScanPP the same input is trivia plus the once-per-file
	// report — which is what a .mi that still has directives in it
	// deserves.
	toks, diags = scan(t, "#import <Foundation/Foundation.h>\nid x;", 0)
	if toks[0].Kind != token.IDENT {
		t.Fatalf("without ScanPP, token 0 = %v, want the 'id'", toks[0].Kind)
	}
	if warnCount(diags) != 1 {
		t.Errorf("want the directive report once: %v", diags)
	}
}

// The @import of §4.4 is a module import and a directive, not a
// preprocessing directive — it has no '#' and survives phase 4.
func TestModuleImportIsADirective(t *testing.T) {
	for _, mode := range []Mode{0, ScanPP} {
		toks, diags := scan(t, "@import Foundation;", mode)
		if toks[0].Kind != token.AT_IMPORT {
			t.Errorf("mode %d: token 0 = %v, want AT_IMPORT", mode, toks[0].Kind)
		}
		if errCount(diags) != 0 {
			t.Errorf("mode %d: %v", mode, diags)
		}
	}
}

func TestScanPPDoesNotBalanceBrackets(t *testing.T) {
	// Real headers define brackets as macros. That is not an error, and
	// a header is not a translation unit — bracket balance is a claim
	// about preprocessed source.
	for _, src := range []string{
		"#define NS_ASSUME_NONNULL_BEGIN _Pragma(\"clang assume_nonnull begin\")",
		"#define BEGIN {\n#define END }",
		"#if 0\n}\n#endif",
	} {
		if _, diags := scan(t, src, ScanPP); errCount(diags) != 0 {
			t.Errorf("%q under ScanPP: %v, want none", src, diags)
		}
	}
	// Without ScanPP the stack still reports, unchanged.
	if _, diags := scan(t, "@implementation Foo {", 0); errCount(diags) != 1 {
		t.Errorf("without ScanPP the stack still reports: %v", diags)
	}
}

func TestScanPPDefersValueErrors(t *testing.T) {
	// A pp-number is not yet a constant, and an excluded group may hold
	// an @ on anything. Both are deferred to the scan above phase 4.
	for _, src := range []string{
		"int a[0779];", "int a[0x1.8];", `char c = '\q';`,
		"#if 0\nid x = @NSFoo;\n#endif",
		"#define BOX(x) @x",
	} {
		if _, diags := scan(t, src, ScanPP); errCount(diags) != 0 {
			t.Errorf("%q under ScanPP: %v, want zero", src, diags)
		}
	}
	// The classification still happens, because phase 4 keys on it.
	toks, _ := scan(t, "@NSFoo", ScanPP)
	if toks[0].Kind != token.AT || toks[1].Kind != token.IDENT {
		t.Errorf("tokens = %v, want the @ and the identifier", toks)
	}
	toks, _ = scan(t, "@interface", ScanPP)
	if toks[0].Kind != token.AT_INTERFACE {
		t.Errorf("token 0 = %v, want AT_INTERFACE in both modes", toks[0].Kind)
	}
}

func TestScanPPPPNumberIsOneToken(t *testing.T) {
	// The pp-number rule: one run of digits, letters, '.', and exponent
	// signs is one token, whatever it classifies as. Phase 4 needs the
	// span intact even when the classification is a diagnosis.
	toks, _ := scan(t, "0779", ScanPP)
	if len(toks) != 2 || toks[0].Kind != token.INT_LIT {
		t.Fatalf("tokens = %v, want one INT_LIT and EOF", toks)
	}
	f := token.NewFile("a.m", []byte("0x1p+4f\n"))
	toks, _ = Scan(f, ScanPP)
	if got := string(f.Slice(toks[0].Pos, toks[0].End)); got != "0x1p+4f" {
		t.Errorf("pp-number span = %q, want the whole run", got)
	}
}

func TestScanPPLineStructure(t *testing.T) {
	// A logical line ends at the next FlagNLBefore, and phase 2 has
	// already spliced backslash-newlines away — so a continued
	// directive is one line, which is what makes multi-line #defines
	// work.
	toks, _ := scan(t, "#define M(a) \\\n  ((a)+1)\nid x;", ScanPP)
	nl := 0
	for _, tk := range toks {
		if tk.Kind != token.EOF && tk.Flags.Has(token.FlagNLBefore) {
			nl++
		}
	}
	if nl != 2 { // the '#', and the 'id' after the directive
		t.Errorf("%d line-opening tokens, want 2 (the '#', and the 'id' after the directive)", nl)
	}
}

// The last alternative of C11 §6.4p1's pp-token grammar — "each
// non-white-space character that cannot be one of the above" — is a
// preprocessing token like any other. A backslash outside a UCN is one.
// The @ is not: Objective-C gives it a punctuator (§2.6).
//
// What such a character is not is a token: §6.4p1's constraint on the
// conversion is phase 7's, so the diagnostic belongs where the
// character survives to and not where it was written. A macro
// replacement list that no translation unit expands may hold one.
func TestScanPPDefersStrayCharacters(t *testing.T) {
	for _, src := range []string{
		`#define NEVER(a) f(#a, a\t)`,
		"#if 0\n`\n#endif",
	} {
		toks, diags := scan(t, src, ScanPP)
		if errCount(diags) != 0 {
			t.Errorf("%q under ScanPP: %v, want zero", src, diags)
		}
		// Quiet, but present: phase 4 has to carry the token, or
		// expanding the macro would lose what phase 7 is meant to
		// reject.
		found := false
		for _, tk := range toks {
			if tk.Kind == token.ILLEGAL {
				found = true
			}
		}
		if !found {
			t.Errorf("%q under ScanPP: no ILLEGAL token, want the pp-token kept", src)
		}
	}
}

// Phase 7 is where it is rejected.
func TestStrayCharactersReportAtPhase7(t *testing.T) {
	for _, src := range []string{`int x = a\t;`, "int x = `;"} {
		if _, diags := scan(t, src, 0); errCount(diags) != 1 {
			t.Errorf("%q: %d errors %v, want one", src, errCount(diags), diags)
		}
	}
}

// A whole small translation unit, as the README's quick start writes
// it: the shapes above in one stream, scanning clean.
func TestGreeter(t *testing.T) {
	const src = `@interface Greeter : NSObject
- (void)greet:(NSString *)name;
@end

@implementation Greeter
- (void)greet:(NSString *)name {
    NSLog(@"Hello, %@!", name);
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        Greeter *greeter = [[Greeter alloc] init];
        [greeter greet:@"World"];
    }
    return 0;
}`
	toks, diags := scan(t, src, 0)
	if errCount(diags) != 0 || warnCount(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	want := map[token.Kind]int{
		token.AT_INTERFACE: 1, token.AT_IMPLEMENTATION: 1, token.AT_END: 2,
		token.AT_AUTORELEASEPOOL: 1, token.OBJC_STRING_LIT: 2, token.AT: 0,
	}
	got := map[token.Kind]int{}
	for _, tk := range toks {
		got[tk.Kind]++
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%v: %d, want %d", k, got[k], n)
		}
	}
}
