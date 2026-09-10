// §5.9 Attributes

__attribute__((unused)) static int unusedGlobal;
__attribute__((deprecated)) void oldFunction(void);
__attribute__((deprecated("use newFunction instead"))) void olderFunction(void);
__attribute__((noreturn)) void diverges(void);
__attribute__((format(printf, 1, 2))) void logf(const char *fmt, ...);
__attribute__((aligned(16))) char alignedBuffer[64];
__attribute__(()) int emptyAttributeList;
__attribute__((unused, deprecated)) int two;
__attribute__((unused)) __attribute__((deprecated)) int twoSpecifiers;

// The __-decorated spelling, which exists so a macro named `packed` cannot
// break a header
struct __attribute__((__packed__)) Packed { char a; int b; };

// A specifier before the tag, and after the closing brace
struct __attribute__((aligned(8))) AlignedStruct { int x; };
struct AlignedTail { int x; } __attribute__((aligned(8)));

// On a declarator rather than in the specifiers: this aligns a and not b
int a __attribute__((aligned(16))), b;

// On an enumerator
enum Availability {
    Current,
    Legacy __attribute__((deprecated)),
};

// The availability attribute, which is on nearly every SDK declaration and
// is not an expression in any grammar
__attribute__((availability(macosx, introduced=10.12.1, deprecated=11.0)))
void availabilityAttribute(void);

// The bracketed spelling, with a scoped AttributeName
[[deprecated]] void bracketed(void);
[[clang::objc_arc]] void scoped(void);
