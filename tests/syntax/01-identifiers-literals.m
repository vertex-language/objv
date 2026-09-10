// §2.1 Identifiers, §2.3 Constants, §2.4 String Literals

// IdentifierNondigit {IdentifierNondigit | Digit}
int camelCase;
int _underscoreStart;
int mixed123;
int $dollar;            // $ is an extension, enabled by default
int \u00C5ngstrom;      // UniversalCharacterName

// IntegerConstant: decimal, octal, hexadecimal, binary
int dec = 42;
int oct = 0777;
int hex = 0xDEADbeef;
int bin = 0b1011;
unsigned long long suffixes = 1u + 2U + 3l + 4L + 5ul + 6llu + 7ULL;

// FloatingConstant
double d1 = 1.5;
double d2 = .5;
double d3 = 1.;
double d4 = 1e10;
double d5 = 1.5e-3;
double d6 = 0x1.8p3;
float f1 = 1.5f;
long double ld = 1.5L;

// CharacterConstant, with every EscapeSequence shape
char c1 = 'a';
char c2 = '\n';
char c3 = '\0';
char c4 = '\x41';
char c5 = '\101';
char c6 = 'é';
int wide = L'a' + u'b' + U'c';

// BooleanConstant: what YES and NO expand to
signed char yes = __objc_yes;
signed char no = __objc_no;

// PlainStringLiteral, with every EncodingPrefix
const char *s1 = "plain";
const char *s2 = u8"utf8";
const void *s3 = u"utf16";
const void *s4 = U"utf32";
const void *s5 = L"wide";

// StringLiteralSequence: adjacent literals concatenate
const char *joined = "one" "two" "three";

// ObjectStringLiteral, and the sequences §6.1 builds from one
id str = @"object";
id joined1 = @"a" @"b";
id joined2 = @"a" "b";
