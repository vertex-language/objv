package token

import (
	"bytes"
	"testing"
)

func TestKeywordCount(t *testing.T) {
	if n := int(c_keyword_end - keyword_beg - 1); n != 44 {
		t.Fatalf("got %d C11 keywords, want 44", n)
	}
	if n := int(objc_keyword_end - objc_keyword_beg - 1); n != 17 {
		t.Fatalf("got %d Objective-C keyword kinds, want 17", n)
	}
	if n := int(directive_end - directive_beg - 1); n != 26 {
		t.Fatalf("got %d directives, want the 26 of §2.5", n)
	}
}

func TestLookup(t *testing.T) {
	for name, want := range map[string]Kind{
		"typedef": TYPEDEF, "_Thread_local": THREAD_LOCAL,
		"_Imaginary": IMAGINARY, "T": IDENT, "Auto": IDENT, "": IDENT,
		"__auto_type": AUTO_TYPE,

		// Objective-C keywords.
		"__block": BLOCK, "__kindof": KINDOF, "__bridge": BRIDGE,
		"__bridge_transfer": BRIDGE_TRANSFER, "__unsafe_unretained": UNSAFE_UNRETAINED,
		"__covariant": COVARIANT, "__attribute__": ATTRIBUTE,
		"__builtin_available": BUILTIN_AVAILABLE, "__ptrauth": PTRAUTH,

		// Alias spellings resolve to the kind they are a spelling of.
		"__typeof": TYPEOF, "__typeof__": TYPEOF, "typeof": TYPEOF,
		"__asm__": ASM, "asm": ASM,
		"__alignof": ALIGNOF, "__alignof__": ALIGNOF, "_Alignof": ALIGNOF,
		"__thread": THREAD_LOCAL,
		"_Nonnull": NONNULL, "__nonnull": NONNULL,
		"_Nullable": NULLABLE, "__nullable": NULLABLE,
		"__null_unspecified": NULL_UNSPECIFIED,

		// §2.3 makes these constants, not keywords.
		"__objc_yes": BOOL_LIT, "__objc_no": BOOL_LIT,

		// Not keywords: typedef names from <objc/objc.h>, a forward
		// -declared class, macros, and the contextual keywords of §2.2.
		"id": IDENT, "Class": IDENT, "SEL": IDENT, "IMP": IDENT, "BOOL": IDENT,
		"Protocol": IDENT, "nil": IDENT, "YES": IDENT,
		"instancetype": IDENT, "nonnull": IDENT, "copy": IDENT,
		"nonatomic": IDENT, "oneway": IDENT, "in": IDENT,
	} {
		if got := Lookup(name); got != want {
			t.Errorf("Lookup(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestLookupDirective(t *testing.T) {
	for name, want := range map[string]Kind{
		"interface": AT_INTERFACE, "implementation": AT_IMPLEMENTATION,
		"end": AT_END, "compatibility_alias": AT_COMPATIBILITY_ALIAS,
		"autoreleasepool": AT_AUTORELEASEPOOL, "selector": AT_SELECTOR,
		"defs": AT_DEFS, "available": AT_AVAILABLE,

		// The set is closed: an @ on anything else is not a directive,
		// which is why an enumeration constant cannot be boxed by name.
		"foo": ILLEGAL, "true": ILLEGAL, "NSUTF8StringEncoding": ILLEGAL,
		"": ILLEGAL, "@interface": ILLEGAL,
	} {
		if got := LookupDirective(name); got != want {
			t.Errorf("LookupDirective(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestString(t *testing.T) {
	for k, want := range map[Kind]string{
		AT_INTERFACE: "@interface", AT_END: "@end", AT: "@",
		BRIDGE_RETAINED: "__bridge_retained", NONNULL: "_Nonnull",
		TYPEOF: "typeof", IDENT: "IDENT", OBJC_STRING_LIT: "OBJC_STRING_LIT",
		Kind(250): "Kind(250)",
	} {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}

func TestClassification(t *testing.T) {
	for _, c := range []struct {
		k                                  Kind
		lit, punct, keyword, directive, oc bool
	}{
		{INT_LIT, true, false, false, false, false},
		{BOOL_LIT, true, false, false, false, false},
		{AT, false, true, false, false, false},
		{WHILE, false, false, true, false, false},
		{BLOCK, false, false, true, false, true},
		{AUTO_TYPE, false, false, true, false, false},
		{AT_TRY, false, false, false, true, false},
		{IDENT, false, false, false, false, false},
	} {
		if c.k.IsLiteral() != c.lit || c.k.IsPunct() != c.punct ||
			c.k.IsKeyword() != c.keyword || c.k.IsDirective() != c.directive ||
			c.k.IsObjCKeyword() != c.oc {
			t.Errorf("%v: literal=%v punct=%v keyword=%v directive=%v objc=%v",
				c.k, c.k.IsLiteral(), c.k.IsPunct(), c.k.IsKeyword(),
				c.k.IsDirective(), c.k.IsObjCKeyword())
		}
	}
}

func TestPrecedence(t *testing.T) {
	order := []Kind{COMMA, LOR, LAND, OR, XOR, AND, EQL, LSS, SHL, ADD, MUL}
	for i := 1; i < len(order); i++ {
		if order[i-1].Precedence() >= order[i].Precedence() {
			t.Errorf("%v (%d) should bind looser than %v (%d)",
				order[i-1], order[i-1].Precedence(), order[i], order[i].Precedence())
		}
	}
	for _, k := range []Kind{ASSIGN, QUESTION, INC, IDENT, NOT, AT, AT_SELECTOR} {
		if k.Precedence() != LowestPrec {
			t.Errorf("%v.Precedence() = %d, want LowestPrec", k, k.Precedence())
		}
	}
}

// SplitAngle is the one place §5.5's angle-bracket lists need a scanned
// token taken apart: NSArray<id<NSCopying>> closes two lists with one
// SHR, because nothing below the parser knows a list is open.
func TestSplitAngle(t *testing.T) {
	for _, c := range []struct {
		in         Kind
		lead, rest Kind
		ok         bool
	}{
		{SHR, GTR, GTR, true},
		{SHR_ASSIGN, GTR, GEQ, true},
		{GEQ, GTR, ASSIGN, true},
		{GTR, GTR, ILLEGAL, false},
		{LSS, LSS, ILLEGAL, false},
	} {
		tk := Token{Kind: c.in, Flags: FlagNLBefore, Pos: 10, End: 10 + Pos(len(c.in.String()))}
		lead, rest, ok := tk.SplitAngle()
		if ok != c.ok {
			t.Errorf("%v: ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			if lead != tk {
				t.Errorf("%v: an unsplit token must come back unchanged", c.in)
			}
			continue
		}
		if lead.Kind != c.lead || rest.Kind != c.rest {
			t.Errorf("%v splits to %v %v, want %v %v", c.in, lead.Kind, rest.Kind, c.lead, c.rest)
		}
		if lead.Pos != tk.Pos || lead.End != tk.Pos+1 || rest.Pos != tk.Pos+1 || rest.End != tk.End {
			t.Errorf("%v: spans %v %v do not tile [%d,%d)", c.in, lead, rest, tk.Pos, tk.End)
		}
		if !lead.Flags.Has(FlagNLBefore) || !rest.Flags.Has(FlagAdjacent) {
			t.Errorf("%v: flags %v %v, want the lead to keep its own and the rest to be adjacent",
				c.in, lead.Flags, rest.Flags)
		}
	}
}

func TestFastPath(t *testing.T) {
	f := NewFile("a.m", []byte("int x = 1;\n"))
	if f.rawLo != nil {
		t.Fatal("expected fast path (nil mapping)")
	}
	if got := string(f.Slice(f.Pos(0), f.Pos(3))); got != "int" {
		t.Errorf("Slice = %q", got)
	}
	if got := string(f.Raw(f.Pos(0), f.Pos(3))); got != "int" {
		t.Errorf("Raw = %q", got)
	}
	if len(f.Diagnostics()) != 0 {
		t.Errorf("unexpected diagnostics: %v", f.Diagnostics())
	}
}

func TestSplice(t *testing.T) {
	f := NewFile("a.m", []byte("in\\\nt x;\n"))
	if got := string(f.Text()); got != "int x;\n" {
		t.Fatalf("Text = %q", got)
	}
	if got := string(f.Slice(f.Pos(0), f.Pos(3))); got != "int" {
		t.Errorf("Slice = %q", got)
	}
	if got := string(f.Raw(f.Pos(0), f.Pos(3))); got != "in\\\nt" {
		t.Errorf("Raw = %q, want the splice widened in", got)
	}
	p := f.Position(f.Pos(2)) // the 't'
	if p.Line != 2 || p.Column != 1 {
		t.Errorf("Position of t = %d:%d, want 2:1", p.Line, p.Column)
	}
	if len(f.Diagnostics()) != 0 {
		t.Errorf("unexpected diagnostics: %v", f.Diagnostics())
	}
}

func TestTrigraph(t *testing.T) {
	f := NewFile("a.m", []byte("??<\n"))
	if got := string(f.Text()); got != "{\n" {
		t.Fatalf("Text = %q", got)
	}
	if got := string(f.Raw(f.Pos(0), f.Pos(1))); got != "??<" {
		t.Errorf("Raw = %q, want the whole trigraph", got)
	}
	ds := f.Diagnostics()
	if len(ds) != 1 || ds[0].Severity != Warn {
		t.Fatalf("want one Warn diagnostic, got %v", ds)
	}
}

func TestTrigraphSplice(t *testing.T) {
	// ??/ becomes backslash in phase 1, then splices in phase 2.
	f := NewFile("a.m", []byte("in??/\nt;\n"))
	if got := string(f.Text()); got != "int;\n" {
		t.Fatalf("Text = %q", got)
	}
}

func TestBetween(t *testing.T) {
	f := NewFile("a.m", []byte("a \\\n b;\n"))
	// tokens: 'a' at [0,1), 'b' at [3,4) in translated text "a  b;\n"
	prev := Token{Kind: IDENT, Pos: f.Pos(0), End: f.Pos(1)}
	next := Token{Kind: IDENT, Pos: f.Pos(3), End: f.Pos(4)}
	if got := f.Between(prev, next); !bytes.Equal(got, []byte(" \\\n ")) {
		t.Errorf("Between = %q, want the splice kept as trivia", got)
	}
}

func TestSortDiagnostics(t *testing.T) {
	ds := []Diagnostic{
		{Pos: 5, End: 6, Message: "b"},
		{Pos: 1, End: 3, Message: "z"},
		{Pos: 5, End: 6, Message: "a"},
		{Pos: 1, End: 2, Message: "y"},
	}
	SortDiagnostics(ds)
	want := []string{"y", "z", "a", "b"}
	for i, m := range want {
		if ds[i].Message != m {
			t.Errorf("diagnostic %d = %q, want %q", i, ds[i].Message, m)
		}
	}
}

func TestDiagnosticPrint(t *testing.T) {
	f := NewFile("a.m", []byte("int x;\n@bogus\n"))
	d := Diagnostic{Pos: f.Pos(7), End: f.Pos(13), Severity: Error, Message: "not a directive"}
	if got, want := d.Print(f), "a.m:2:1: error: not a directive"; got != want {
		t.Errorf("Print = %q, want %q", got, want)
	}
}
