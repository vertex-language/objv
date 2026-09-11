// The GNU C an Objective-C program is written in even when it thinks it is
// writing C.
//
// _Generic is how <tgmath.h> and half of Apple's headers dispatch on a type.
// __attribute__((constructor)) is how a unit registers itself with a table
// nobody passed it a pointer to. A variable-length array is how a program
// sizes a buffer from something it read. None of the three is Objective-C,
// all three are in Objective-C programs, and each of them fails silently if
// a compiler drops it: the wrong arm, the missing registration, the buffer
// that is eight bytes long.

#import <Foundation/Foundation.h>
#include <string.h>

// ---- _Generic ----

// The shape <tgmath.h> is: one name, one association per type. Which one the
// expansion means is decided entirely by the controlling expression's type,
// and getting it wrong is silent — the program computes something, just not
// with the function it named.
#define kindOf(x) _Generic((x), \
    char: "char",               \
    short: "short",             \
    int: "int",                 \
    long: "long",               \
    unsigned: "unsigned",       \
    float: "float",             \
    double: "double",           \
    char *: "string",           \
    const char *: "const string", \
    id: "object",               \
    default: "other")

// Selection on the *promoted* type is not what happens: the controlling
// expression keeps its own type, lvalue conversion and nothing more. A
// `char` selects the char arm, not the int one.
#define widthOf(x) _Generic((x), \
    char: 1, short: 2, int: 4, long: 8, default: 0)

// An array and a pointer are different types, which is the distinction a
// length macro lives on: `sizeof` is the answer for one and `strlen` for the
// other, and only the type tells them apart. Every association is typed —
// C11 requires all of them to be valid expressions — but only the selected
// one is evaluated, so the other's call never happens.
#define measure(x) _Generic((x), \
    char *: strlen((const char *)(x)), \
    const char *: strlen((const char *)(x)), \
    default: sizeof(x) - 1)

// ---- constructors ----

static NSMutableArray<NSString *> *gOrder;
static int gRegistered;

// Priority 101 runs before the unadorned ones, which run in the order they
// were written. The array has to exist before anything appends to it, which
// is what the priority is for.
__attribute__((constructor(101))) static void makeOrder(void) {
    gOrder = [[NSMutableArray alloc] init];
}

__attribute__((constructor)) static void registerFirst(void) {
    [gOrder addObject:@"first"];
    gRegistered++;
}

__attribute__((constructor)) static void registerSecond(void) {
    [gOrder addObject:@"second"];
    gRegistered++;
}

// The attribute on a declaration and the body somewhere else, which is how
// it is written when the function is also called by name.
static void sayGoodbye(void) __attribute__((destructor));
static void sayGoodbye(void) { printf("destructor: goodbye\n"); }

__attribute__((destructor)) static void sayLast(void) {
    printf("destructor: last\n");
}

// ---- variable-length arrays ----

// The length is read once, where the declaration stands. A length expression
// with a side effect runs exactly once however many times the array is used.
static int sideEffects;
static int nextLength(void) { sideEffects++; return 6; }

static NSString *reverse(const char *s) {
    size_t n = strlen(s);
    char buf[n + 1];
    for (size_t i = 0; i < n; i++) buf[i] = s[n - 1 - i];
    buf[n] = '\0';
    // sizeof a variably modified object is evaluated here and is the size
    // the object has, not a pointer's width.
    if (sizeof buf != n + 1) return @"sizeof disagrees";
    return [NSString stringWithUTF8String:buf];
}

// A loop body that allocates has to give the space back, or two hundred
// thousand iterations exhaust the stack.
static long churn(int rounds) {
    long total = 0;
    for (int i = 0; i < rounds; i++) {
        int n = (i % 64) + 1;
        int a[n];
        for (int j = 0; j < n; j++) a[j] = j;
        for (int j = 0; j < n; j++) total += a[j];
    }
    return total;
}

int main(void) {
    @autoreleasepool {
        // The constructors ran before this line.
        printf("registered: %d\n", gRegistered);
        for (NSString *s in gOrder) printf("order: %s\n", s.UTF8String);

        char c = 'x';
        short h = 1;
        int i = 2;
        long l = 3;
        unsigned u = 4;
        float f = 5;
        double d = 6;
        char *s = "seven";
        const char *cs = "eight";
        id o = @"nine";
        printf("%s %s %s %s %s %s %s %s %s %s\n",
               kindOf(c), kindOf(h), kindOf(i), kindOf(l), kindOf(u),
               kindOf(f), kindOf(d), kindOf(s), kindOf(cs), kindOf(o));
        printf("%d %d %d %d %d\n",
               widthOf(c), widthOf(h), widthOf(i), widthOf(l), widthOf(f));

        // The controlling expression is not evaluated: a null dereference
        // in it is well defined, because nothing reads it.
        printf("%s\n", kindOf(*(double *)0));

        char fixed[] = "abcdef";
        printf("%zu %zu\n", measure(s), measure(fixed));

        printf("%s\n", reverse("objective").UTF8String);
        printf("%s\n", reverse("c").UTF8String);

        int n = nextLength();
        char room[n];
        memset(room, '!', sizeof room);
        printf("%d %zu %.*s\n", sideEffects, sizeof room, (int)sizeof room, room);

        printf("%ld\n", churn(200000));
    }
    return 0;
}
