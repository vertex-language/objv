package preprocessor

import (
	"fmt"
	"time"

	"github.com/vertex-language/objv/token"
)

// installPredefines defines the macros the compiler itself supplies, then
// applies the caller's -D and -U in order.
//
// Everything target-dependent — __APPLE__, __MACH__, __CHAR_BIT__,
// __SIZEOF_LONG__, __INT_MAX__ and kin — arrives through Config.Predefines,
// computed by the objv package from a target model. This package never learns
// what a target is, the same inversion that keeps sysroot out of phase 4 and
// that puts __has_feature's answer in Config rather than in a table here.
func (p *Preprocessor) installPredefines() {
	p.internal = true
	defer func() { p.internal = false }()

	std := func(name string, b Builtin) {
		m := &Macro{Name: name, ObjLike: true, Builtin: b}
		p.macros.Define(m)
	}
	std("__FILE__", BuiltinFile)
	std("__LINE__", BuiltinLine)
	std("__DATE__", BuiltinDate)
	std("__TIME__", BuiltinTime)
	// __COUNTER__ is gcc's, and the only way to build a name that is unique
	// per expansion — which is what a macro that declares something needs
	// when it may be used twice in one scope. Objective-C leans on it harder
	// than C does: the associated-object, keypath and logging macros that
	// every large Objective-C codebase carries are written with it.
	std("__COUNTER__", BuiltinCounter)

	hosted := "0"
	if p.cfg.Hosted {
		hosted = "1"
	}
	for _, d := range [][2]string{
		{"__STDC__", "1"},
		{"__STDC_VERSION__", p.cfg.Std.Version()},
		{"__STDC_HOSTED__", hosted},
		{"__OBJV__", "1"},

		// The two macros that say this is Objective-C, and which one.
		//
		// __OBJC__ is what every dual-language header keys on: <objc/objc.h>
		// declares Protocol with @class under it, <sys/cdefs.h> and every
		// CoreFoundation header change what they declare, and a .h shared
		// between C and Objective-C sources has no other way to tell which
		// it is being read as.
		//
		// __OBJC2__ says the runtime is the modern one — non-fragile ivars,
		// synthesized properties, and the metadata layout runtime/ emits. It
		// is a fact about what objv generates rather than a claim about the
		// host: the legacy 32-bit runtime is not a target objv has, which is
		// also why @defs (§2.5) is rejected.
		{"__OBJC__", "1"},
		{"__OBJC2__", "1"},

		// gcc's name for the same conclusion, still tested by headers older
		// than the runtime split: declared properties exist.
		{"OBJC_NEW_PROPERTIES", "1"},

		// §6.10.8.3's conditional feature macros. Each says objv does not
		// implement an optional part of C, and each is defined because that
		// is true today rather than because it is convenient: a program that
		// tests them is entitled to be told, and a program that does not is
		// entitled to have the feature work.
		{"__STDC_NO_COMPLEX__", "1"},
		{"__STDC_NO_THREADS__", "1"},

		// objv claims GCC compatibility because the Objective-C it has to
		// compile is the Objective-C that exists, and that language is
		// written for gcc and clang. A platform's own headers are the first
		// and least avoidable case: Darwin's <sys/cdefs.h> greets a compiler
		// that does not define __GNUC__ with "#warning Unsupported compiler
		// detected", and then <libkern/_OSByteOrder.h> declines to declare
		// the byte swaps that htons expands to, so ordinary networking code
		// does not compile.
		//
		// The version is the one clang reports. It is not a claim to be gcc
		// 4.2 — nothing is — but the number every header's feature test was
		// written against, and raising it only opts into newer extensions.
		{"__GNUC__", "4"},
		{"__GNUC_MINOR__", "2"},
		{"__GNUC_PATCHLEVEL__", "1"},

		// And says which `inline` it has, because saying __GNUC__ without
		// this one asks for the other. Darwin's <sys/cdefs.h> spells
		// __header_inline as plain `inline` for a compiler that sets this,
		// and as `extern __inline` — gcc 89's, meaning "inline definition
		// only" — for a compiler that does not. objv's inline is C99's, in
		// which `extern inline` means the opposite: it *does* provide an
		// external definition. So without this every unused inline in a
		// system header would come out as a weak definition in every object
		// that imported it, dragging its own undefined references along.
		{"__GNUC_STDC_INLINE__", "1"},
	} {
		p.definePlain(d[0], d[1])
	}

	// Interface Builder's four keywords, which are the compiler's macros and
	// not any framework's: no SDK header defines them, and a nib-backed class
	// will not compile without them.
	//
	// clang spells them as attributes it records for Interface Builder to
	// read back out of the AST — IBAction as the notorious
	// `void)__attribute__((ibaction)`. objv emits no such metadata, so it
	// defines each as what the declaration means with the metadata removed.
	// The code compiles the same and means the same; what is lost is a fact
	// about the source that only a nib editor ever wanted.
	for _, d := range [][2]string{
		{"IBAction", "void"},
		{"IBOutlet", ""},
		{"IBInspectable", ""},
		{"IB_DESIGNABLE", ""},
	} {
		p.definePlain(d[0], d[1])
	}
	p.definePlain("IBOutletCollection(ClassName)", "")

	// The caller's -D and -U are the program's, and are held to §6.10.8p2
	// like any other definition: a command line may not redefine __FILE__.
	p.internal = false
	for _, d := range p.cfg.Predefines {
		switch d.Kind {
		case PredefineUndef:
			p.macros.Undef(d.Text)
		default:
			p.defineText("<command-line>", d.Text)
		}
	}
}

// definePlain defines one compiler-supplied macro. It goes through the same
// #define grammar a directive uses, exactly as -D does: a body that is minted
// by hand is a body whose tokens were never scanned, and `IBAction` would
// then reach the parser as an identifier spelled "void" rather than as the
// keyword. An empty value defines the macro with no replacement list, which
// is the difference between `#define X 1` and `#define X` and is a difference
// a program can test.
func (p *Preprocessor) definePlain(name, value string) {
	p.defineText("<built-in>", name+"="+value)
}

// defineText runs a -D spelling through the same #define grammar a directive
// uses, so the two cannot drift apart. `-D NAME` defines NAME as 1. The file
// name is what a diagnostic about the definition points at: <built-in> for
// the macros above, <command-line> for the caller's.
func (p *Preprocessor) defineText(from, text string) {
	src := text
	if i := indexByte(text, '='); i >= 0 {
		src = text[:i] + " " + text[i+1:]
	} else {
		src = text + " 1"
	}
	f := token.NewFile(from, []byte(src+"\n"))
	toks, diags := scanPP(f)
	for _, d := range diags {
		p.fromToken(f, d)
	}
	org := &Origin{File: f}
	line := p.wrap(toks, org)
	p.doDefine(&reader{org: org}, trimEOF(line), Site{Origin: org, Pos: f.Pos(0), End: f.Pos(1)})
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trimEOF(ts []Token) []Token {
	if n := len(ts); n > 0 && ts[n-1].Kind == token.EOF {
		return ts[:n-1]
	}
	return ts
}

// builtin computes the replacement list for a computed macro. Each returns
// exactly one token, minted into the generated arena, so the result is an
// ordinary span in an ordinary position space.
//
// __DATE__ and __TIME__ read Config.Epoch, never a clock: SOURCE_DATE_EPOCH is
// the same contract objv's object writers hold to, and there is no other
// mode — no --deterministic flag, because there is nothing to switch off.
func (p *Preprocessor) builtin(m *Macro, at Token) []Token {
	var t Token
	switch m.Builtin {
	case BuiltinFile:
		t = p.gen.Mint(token.STRING_LIT, fmt.Sprintf("%q", p.currentFileName(at)))
	case BuiltinLine:
		t = p.gen.Mint(token.INT_LIT, fmt.Sprint(p.currentLine(at)))
	case BuiltinDate:
		t = p.gen.Mint(token.STRING_LIT, `"`+p.epochFormat("Jan  2 2006")+`"`)
	case BuiltinTime:
		t = p.gen.Mint(token.STRING_LIT, `"`+p.epochFormat("15:04:05")+`"`)
	case BuiltinCounter:
		t = p.gen.Mint(token.INT_LIT, fmt.Sprint(p.counter))
		p.counter++
	default:
		return nil
	}
	t.Flags = at.Flags & token.FlagAdjacent
	t.Exp = &Expansion{Macro: m.Name, Use: at.Site(), Outer: at.Exp}
	return []Token{t}
}

func (p *Preprocessor) epochFormat(layout string) string {
	now := p.cfg.Now()
	if now.IsZero() {
		now = time.Unix(0, 0).UTC()
	}
	return now.Format(layout)
}

// currentFileName is the path as written, never made absolute: an absolute
// path is the build machine leaking into the output.
func (p *Preprocessor) currentFileName(at Token) string {
	if p.fileName != "" {
		return p.fileName
	}
	s := at.Site()
	if s.Origin != nil {
		return s.Origin.Name()
	}
	return "<unknown>"
}

func (p *Preprocessor) currentLine(at Token) int {
	return p.physicalLine(at.Site()) + p.lineDelta
}

func (p *Preprocessor) physicalLine(s Site) int {
	if s.Origin == nil || s.Origin.File == nil || !s.Pos.IsValid() {
		return 0
	}
	return s.Origin.File.Position(s.Pos).Line
}
