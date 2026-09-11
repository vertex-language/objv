package token

// Kind identifies the lexical class of a token.
type Kind uint8

const (
	ILLEGAL Kind = iota
	EOF
	COMMENT

	IDENT

	literal_beg
	INT_LIT
	FLOAT_LIT
	CHAR_LIT
	STRING_LIT // "…", u8"…", u"…", U"…", L"…"
	OBJC_STRING_LIT
	// BOOL_LIT is __objc_yes / __objc_no, which YES and NO expand to.
	// §2.3 makes it a Constant and not a keyword, so that a boxed
	// boolean is distinguishable from a boxed integer (§6.8).
	BOOL_LIT
	literal_end

	punct_beg
	LBRACK // [
	RBRACK // ]
	LPAREN // (
	RPAREN // )
	LBRACE // {
	RBRACE // }

	PERIOD // .
	ARROW  // ->
	INC    // ++
	DEC    // --

	AND   // &
	MUL   // *
	ADD   // +
	SUB   // -
	TILDE // ~
	NOT   // !

	QUO  // /
	REM  // %
	SHL  // <<
	SHR  // >>
	LSS  // <
	GTR  // >
	LEQ  // <=
	GEQ  // >=
	EQL  // ==
	NEQ  // !=
	XOR  // ^ — also the block introducer of §5.7 and §6.9
	OR   // |
	LAND // &&
	LOR  // ||

	QUESTION // ?
	// COLON is also the whole of §2.6's :: punctuator, which is two
	// adjacent COLON tokens and not a kind of its own. Munching the
	// pair would break @selector(a::) — §6.4's SelectorName is pieces
	// and colons with no parameters to name, so a selector whose
	// second piece is nameless puts two colons together. The scoped
	// AttributeName of §5.9 that :: otherwise serves is a C++ import
	// reached only through the bracketed attribute form, and the
	// parser recognizes it there from FlagAdjacent.
	COLON    // :
	SEMI     // ;
	ELLIPSIS // ...

	ASSIGN     // =
	MUL_ASSIGN // *=
	QUO_ASSIGN // /=
	REM_ASSIGN // %=
	ADD_ASSIGN // +=
	SUB_ASSIGN // -=
	SHL_ASSIGN // <<=
	SHR_ASSIGN // >>=
	AND_ASSIGN // &=
	XOR_ASSIGN // ^=
	OR_ASSIGN  // |=

	COMMA    // ,
	HASH     // #
	HASHHASH // ##
	AT       // @ — a bare one: §6.8's boxing and literal forms
	punct_end

	// The 44 keywords of C11 §6.4.1. _Imaginary is reserved even in
	// implementations without Annex G.
	keyword_beg
	AUTO
	BREAK
	CASE
	CHAR
	CONST
	CONTINUE
	DEFAULT
	DO
	DOUBLE
	ELSE
	ENUM
	EXTERN
	FLOAT
	FOR
	GOTO
	IF
	INLINE
	INT
	LONG
	REGISTER
	RESTRICT
	RETURN
	SHORT
	SIGNED
	SIZEOF
	STATIC
	STRUCT
	SWITCH
	TYPEDEF
	UNION
	UNSIGNED
	VOID
	VOLATILE
	WHILE
	ALIGNAS       // _Alignas
	ALIGNOF       // _Alignof
	ATOMIC        // _Atomic
	BOOL          // _Bool
	COMPLEX       // _Complex
	GENERIC       // _Generic
	IMAGINARY     // _Imaginary
	NORETURN      // _Noreturn
	STATIC_ASSERT // _Static_assert
	THREAD_LOCAL  // _Thread_local
	c_keyword_end

	// The Objective-C keywords of §2.2. Every one is spelled in the
	// reserved __ or _Capital namespace, so recognizing it takes
	// nothing away from a conforming C program — which is why the
	// language spells them this way and not, say, `block`.
	//
	// id, Class, SEL, IMP, and BOOL are deliberately absent: they are
	// typedef names from <objc/objc.h> and reach the grammar through
	// TypedefName (§5.3). Protocol is absent for the same reason, by
	// way of ClassName (§4.1). nil, Nil, YES, and NO are macros.
	objc_keyword_beg
	BLOCK             // __block
	KINDOF            // __kindof
	BRIDGE            // __bridge
	BRIDGE_RETAINED   // __bridge_retained
	BRIDGE_TRANSFER   // __bridge_transfer
	STRONG            // __strong
	WEAK              // __weak
	UNSAFE_UNRETAINED // __unsafe_unretained
	AUTORELEASING     // __autoreleasing
	COVARIANT         // __covariant
	CONTRAVARIANT     // __contravariant
	NONNULL           // _Nonnull, __nonnull
	NULLABLE          // _Nullable, __nullable, _Nullable_result
	NULL_UNSPECIFIED  // _Null_unspecified, __null_unspecified
	BUILTIN_AVAILABLE // __builtin_available
	PTRAUTH           // __ptrauth
	ATTRIBUTE         // __attribute__
	objc_keyword_end

	// The extension keywords of §2.2. These are not in C11 §6.4.1;
	// each is in routine use in Objective-C code or in the Cocoa
	// headers, and each has alias spellings listed in aliases below.
	ASM       // asm, __asm, __asm__
	TYPEOF    // typeof, __typeof, __typeof__
	AUTO_TYPE // __auto_type
	EXTENSION // __extension__

	// Two extension *types*, which the C substrate carries because the
	// platform's headers declare things in terms of them and no conforming
	// spelling exists for either. clang provides both on every 64-bit
	// target: <mach/arm/_structs.h> declares the NEON register file as
	// __uint128_t __v[32], and <math.h> declares half-precision entry
	// points in terms of _Float16.
	INT128  // __int128
	FLOAT16 // _Float16
	keyword_end

	// The @-directives of §2.5. Each is one token: the @ punctuator
	// followed by an identifier drawn from this closed set, which is
	// not subject to ordinary identifier lookup. An @ followed by any
	// other identifier is ill-formed, which is also why an enumeration
	// constant cannot be boxed by writing @ before its name (§6.8).
	directive_beg
	AT_INTERFACE
	AT_IMPLEMENTATION
	AT_PROTOCOL
	AT_END
	AT_CLASS
	AT_COMPATIBILITY_ALIAS
	AT_IMPORT
	AT_PROPERTY
	AT_SYNTHESIZE
	AT_DYNAMIC
	AT_REQUIRED
	AT_OPTIONAL
	AT_PRIVATE
	AT_PROTECTED
	AT_PUBLIC
	AT_PACKAGE
	AT_SELECTOR
	AT_ENCODE
	AT_DEFS
	AT_AVAILABLE
	AT_TRY
	AT_CATCH
	AT_FINALLY
	AT_THROW
	AT_SYNCHRONIZED
	AT_AUTORELEASEPOOL
	directive_end
)

var names = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:           "IDENT",
	INT_LIT:         "INT_LIT",
	FLOAT_LIT:       "FLOAT_LIT",
	CHAR_LIT:        "CHAR_LIT",
	STRING_LIT:      "STRING_LIT",
	OBJC_STRING_LIT: "OBJC_STRING_LIT",
	BOOL_LIT:        "BOOL_LIT",

	LBRACK: "[",
	RBRACK: "]",
	LPAREN: "(",
	RPAREN: ")",
	LBRACE: "{",
	RBRACE: "}",

	PERIOD: ".",
	ARROW:  "->",
	INC:    "++",
	DEC:    "--",

	AND:   "&",
	MUL:   "*",
	ADD:   "+",
	SUB:   "-",
	TILDE: "~",
	NOT:   "!",

	QUO:  "/",
	REM:  "%",
	SHL:  "<<",
	SHR:  ">>",
	LSS:  "<",
	GTR:  ">",
	LEQ:  "<=",
	GEQ:  ">=",
	EQL:  "==",
	NEQ:  "!=",
	XOR:  "^",
	OR:   "|",
	LAND: "&&",
	LOR:  "||",

	QUESTION: "?",
	COLON:    ":",
	SEMI:     ";",
	ELLIPSIS: "...",

	ASSIGN:     "=",
	MUL_ASSIGN: "*=",
	QUO_ASSIGN: "/=",
	REM_ASSIGN: "%=",
	ADD_ASSIGN: "+=",
	SUB_ASSIGN: "-=",
	SHL_ASSIGN: "<<=",
	SHR_ASSIGN: ">>=",
	AND_ASSIGN: "&=",
	XOR_ASSIGN: "^=",
	OR_ASSIGN:  "|=",

	COMMA:    ",",
	HASH:     "#",
	HASHHASH: "##",
	AT:       "@",

	AUTO:          "auto",
	BREAK:         "break",
	CASE:          "case",
	CHAR:          "char",
	CONST:         "const",
	CONTINUE:      "continue",
	DEFAULT:       "default",
	DO:            "do",
	DOUBLE:        "double",
	ELSE:          "else",
	ENUM:          "enum",
	EXTERN:        "extern",
	FLOAT:         "float",
	FOR:           "for",
	GOTO:          "goto",
	IF:            "if",
	INLINE:        "inline",
	INT:           "int",
	LONG:          "long",
	REGISTER:      "register",
	RESTRICT:      "restrict",
	RETURN:        "return",
	SHORT:         "short",
	SIGNED:        "signed",
	SIZEOF:        "sizeof",
	STATIC:        "static",
	STRUCT:        "struct",
	SWITCH:        "switch",
	TYPEDEF:       "typedef",
	UNION:         "union",
	UNSIGNED:      "unsigned",
	VOID:          "void",
	VOLATILE:      "volatile",
	WHILE:         "while",
	ALIGNAS:       "_Alignas",
	ALIGNOF:       "_Alignof",
	ATOMIC:        "_Atomic",
	BOOL:          "_Bool",
	COMPLEX:       "_Complex",
	GENERIC:       "_Generic",
	IMAGINARY:     "_Imaginary",
	NORETURN:      "_Noreturn",
	STATIC_ASSERT: "_Static_assert",
	THREAD_LOCAL:  "_Thread_local",

	BLOCK:             "__block",
	KINDOF:            "__kindof",
	BRIDGE:            "__bridge",
	BRIDGE_RETAINED:   "__bridge_retained",
	BRIDGE_TRANSFER:   "__bridge_transfer",
	STRONG:            "__strong",
	WEAK:              "__weak",
	UNSAFE_UNRETAINED: "__unsafe_unretained",
	AUTORELEASING:     "__autoreleasing",
	COVARIANT:         "__covariant",
	CONTRAVARIANT:     "__contravariant",
	NONNULL:           "_Nonnull",
	NULLABLE:          "_Nullable",
	NULL_UNSPECIFIED:  "_Null_unspecified",
	BUILTIN_AVAILABLE: "__builtin_available",
	PTRAUTH:           "__ptrauth",
	ATTRIBUTE:         "__attribute__",

	ASM:       "asm",
	TYPEOF:    "typeof",
	AUTO_TYPE: "__auto_type",
	EXTENSION: "__extension__",
	INT128:    "__int128",
	FLOAT16:   "_Float16",

	AT_INTERFACE:           "@interface",
	AT_IMPLEMENTATION:      "@implementation",
	AT_PROTOCOL:            "@protocol",
	AT_END:                 "@end",
	AT_CLASS:               "@class",
	AT_COMPATIBILITY_ALIAS: "@compatibility_alias",
	AT_IMPORT:              "@import",
	AT_PROPERTY:            "@property",
	AT_SYNTHESIZE:          "@synthesize",
	AT_DYNAMIC:             "@dynamic",
	AT_REQUIRED:            "@required",
	AT_OPTIONAL:            "@optional",
	AT_PRIVATE:             "@private",
	AT_PROTECTED:           "@protected",
	AT_PUBLIC:              "@public",
	AT_PACKAGE:             "@package",
	AT_SELECTOR:            "@selector",
	AT_ENCODE:              "@encode",
	AT_DEFS:                "@defs",
	AT_AVAILABLE:           "@available",
	AT_TRY:                 "@try",
	AT_CATCH:               "@catch",
	AT_FINALLY:             "@finally",
	AT_THROW:               "@throw",
	AT_SYNCHRONIZED:        "@synchronized",
	AT_AUTORELEASEPOOL:     "@autoreleasepool",
}

// String returns the keyword, directive, or punctuator spelling, or
// the class name for kinds with no fixed spelling (IDENT, INT_LIT, …).
func (k Kind) String() string {
	if int(k) < len(names) && names[k] != "" {
		return names[k]
	}
	return "Kind(" + itoa(int(k)) + ")"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

var keywords = func() map[string]Kind {
	m := make(map[string]Kind, keyword_end-keyword_beg)
	for k := keyword_beg + 1; k < keyword_end; k++ {
		switch k {
		case c_keyword_end, objc_keyword_beg, objc_keyword_end:
			continue // markers between the blocks, not keywords
		}
		m[names[k]] = k
	}
	return m
}()

// aliases are alternative spellings of a keyword already in the table.
// The grammar lists most of them as alternatives of one production —
// TypeofKeyword is any of typeof, __typeof, __typeof__ (§5.3), and a
// NullabilityQualifier is either the _Capital or the __lowercase form
// (§5.6) — so an alias means exactly what its keyword means, and
// resolving it here, at the one place a spelling becomes a kind, keeps
// every place that handles the kind from having to know there was more
// than one way to write it.
//
// The underscore-free nullability spellings (nonnull, nullable,
// null_unspecified) are not here. They are contextual: identifiers
// everywhere except inside a MethodType (§4.7) and a
// PropertyAttributeList (§4.8), and only the parser knows it is in one.
var aliases = map[string]Kind{
	"__asm":    ASM,
	"__asm__":  ASM,
	"__typeof": TYPEOF,

	"__typeof__": TYPEOF,

	// gcc accepts __attribute__ with the trailing underscores dropped, and
	// so the SDK writes it that way where a macro would otherwise have to
	// care: <NSLayoutAnchor.h> defines its export macro as
	// `extern __attribute((visibility("default")))`.
	"__attribute": ATTRIBUTE,

	// gcc's names for _Alignof. Its operand is a type name, as
	// _Alignof's is.
	"__alignof":   ALIGNOF,
	"__alignof__": ALIGNOF,

	// gcc's name for _Thread_local, which predates it. §5.1 lists both
	// as StorageClassSpecifier alternatives and calls this one the
	// extension spelling of the other.
	"__thread": THREAD_LOCAL,

	// §5.6's second spelling of each NullabilityQualifier. Both forms
	// are in the Cocoa headers, since NS_ASSUME_NONNULL_BEGIN and the
	// audited-region macros expand to whichever the SDK was written
	// against.
	"__nonnull":  NONNULL,
	"__nullable": NULLABLE,

	// clang's newer spelling, which means _Nullable and additionally tells
	// a Swift importer that the null case is an error result rather than an
	// ordinary value. That distinction is Swift's; to Objective-C the type
	// is nullable, which is what this resolves to. Foundation writes it on
	// every completion handler that can fail.
	"_Nullable_result":   NULLABLE,
	"__null_unspecified": NULL_UNSPECIFIED,

	// gcc's double-underscore spellings of the C keywords. They exist so
	// a header can use a keyword in a translation unit compiled with
	// -ansi, where the keyword itself would not be one, and they never
	// went away: Apple's own headers write `extern __inline
	// __attribute__((__gnu_inline__))` more than a hundred times in
	// Foundation's transitive closure alone.
	"__inline":     INLINE,
	"__inline__":   INLINE,
	"__const":      CONST,
	"__const__":    CONST,
	"__volatile":   VOLATILE,
	"__volatile__": VOLATILE,
	"__signed":     SIGNED,
	"__signed__":   SIGNED,
	"__restrict":   RESTRICT,
	"__restrict__": RESTRICT,
	"__complex":    COMPLEX,
	"__complex__":  COMPLEX,
}

// constants are the spellings §2.3 makes a BooleanConstant rather than
// a keyword or an identifier. YES and NO expand to them, and they exist
// so a boolean literal can be told from an integer one when boxed.
var constants = map[string]Kind{
	"__objc_yes": BOOL_LIT,
	"__objc_no":  BOOL_LIT,
}

// Lookup maps an identifier spelling to its keyword kind, to BOOL_LIT
// for the two boolean constants, or to IDENT. Typedef-ness, class
// names, protocol names, and the contextual keywords of §2.2 are the
// parser's call, not Lookup's.
func Lookup(name string) Kind {
	if k, ok := keywords[name]; ok {
		return k
	}
	if k, ok := aliases[name]; ok {
		return k
	}
	if k, ok := constants[name]; ok {
		return k
	}
	return IDENT
}

var directives = func() map[string]Kind {
	m := make(map[string]Kind, directive_end-directive_beg-1)
	for k := directive_beg + 1; k < directive_end; k++ {
		m[names[k][1:]] = k // without the @
	}
	return m
}()

// LookupDirective maps the identifier following an @ to its directive
// kind, or ILLEGAL. The set is closed (§2.5): an @ followed by anything
// else is not a directive.
func LookupDirective(name string) Kind {
	if k, ok := directives[name]; ok {
		return k
	}
	return ILLEGAL
}

func (k Kind) IsLiteral() bool   { return literal_beg < k && k < literal_end }
func (k Kind) IsPunct() bool     { return punct_beg < k && k < punct_end }
func (k Kind) IsKeyword() bool   { return keyword_beg < k && k < keyword_end }
func (k Kind) IsDirective() bool { return directive_beg < k && k < directive_end }

// IsObjCKeyword reports whether k is one of the Objective-C keywords
// of §2.2, as opposed to a C11 or extension keyword.
func (k Kind) IsObjCKeyword() bool { return objc_keyword_beg < k && k < objc_keyword_end }

// Operator precedence for the ten binary levels of §6.6, plus COMMA
// below them. Assignment and ?: are right-associative and not driven
// by this table.
const (
	LowestPrec  = 0 // non-binary operators
	HighestPrec = 11
)

func (k Kind) Precedence() int {
	switch k {
	case COMMA:
		return 1
	case LOR:
		return 2
	case LAND:
		return 3
	case OR:
		return 4
	case XOR:
		return 5
	case AND:
		return 6
	case EQL, NEQ:
		return 7
	case LSS, GTR, LEQ, GEQ:
		return 8
	case SHL, SHR:
		return 9
	case ADD, SUB:
		return 10
	case MUL, QUO, REM:
		return 11
	}
	return LowestPrec
}
