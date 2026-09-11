// The three quiet ones: an anonymous union's designated initializer, a
// conditional between two pointers that differ only in const, and a
// subscript on id.
//
// Each of them is something a program writes without thinking about it.
// `{ .tag = 1, .d = 2.5 }` is how a tagged union is initialized and the
// only way to say which arm. `e ? e.localizedDescription.UTF8String :
// "none"` is how every fallback for a string-returning method is written.
// `d[@"k"]` on an id is what a JSON walker does on every line. The first
// wrote nothing and said nothing; the other two were rejected outright.

#import <Foundation/Foundation.h>
#include <string.h>

// ---- anonymous members ----

// The shape <mach/message.h>, <netinet/in.h> and every tagged union in every
// protocol header are written in: a discriminant, and an unnamed union whose
// members are reached as if they were the outer struct's.
typedef enum { VInt, VReal, VPair, VText } VKind;

typedef struct {
    VKind kind;
    union {
        long i;
        double d;
        struct { short lo, hi; };   // anonymous, two levels down
        const char *s;
    };
} Value;

static const Value kDefaults[] = {
    { .kind = VInt,  .i = 42 },
    { .kind = VReal, .d = 2.5 },
    { .kind = VPair, .lo = 3, .hi = 4 },
    { .kind = VText, .s = "static" },
};

static void show(Value v) {
    switch (v.kind) {
    case VInt:  printf("int %ld\n", v.i); break;
    case VReal: printf("real %g\n", v.d); break;
    case VPair: printf("pair %d %d\n", v.lo, v.hi); break;
    case VText: printf("text %s\n", v.s); break;
    }
}

// ---- conditional between differently qualified pointers ----

// §6.5.15p6: the result is a pointer to the composite type, qualified with
// the union of both pointees' qualifiers. A const char * arm and a char *
// arm give const char *, not an error.
static const char *describe(NSError *e, char *fallback) {
    return e ? e.domain.UTF8String : fallback;
}

// A pointer to void and a pointer to anything give a pointer to void, which
// is what every generic table lookup relies on.
static const void *pick(int flag, const int *a, void *b) {
    return flag ? a : b;
}

// ---- subscripting an untyped receiver ----

// A JSON walker holds id and subscripts it, because what it holds is
// whatever was in the file. The runtime resolves the send; the compiler's
// job is to let the line through and give it id.
static NSString *dig(id node, NSArray *path) {
    for (id step in path) {
        if (node == nil) return @"(missing)";
        node = [step isKindOfClass:NSNumber.class]
             ? node[[step unsignedIntegerValue]]
             : node[step];
    }
    return [node description];
}

int main(void) {
    @autoreleasepool {
        for (size_t i = 0; i < sizeof kDefaults / sizeof kDefaults[0]; i++) {
            show(kDefaults[i]);
        }

        // The same thing built on the stack rather than in the image.
        Value local = { .kind = VReal, .d = 0.125 };
        show(local);
        local = (Value){ .kind = VPair, .lo = -1, .hi = 7 };
        show(local);
        local.i = 9; local.kind = VInt;
        show(local);

        // Writing through one arm of the union and reading another is what
        // the union is for; the offsets have to agree.
        Value bits = { .kind = VPair, .i = 0 };
        bits.lo = 0x0102;
        bits.hi = 0x0304;
        printf("%lx\n", (unsigned long)bits.i);

        char mutable_[] = "mutable";
        printf("%s\n", describe(nil, mutable_));
        NSError *e = nil;
        [NSString stringWithContentsOfFile:@"/nonexistent/path" encoding:NSUTF8StringEncoding error:&e];
        printf("%s %ld\n", describe(e, mutable_), (long)e.code);

        int one = 1;
        printf("%d\n", *(const int *)pick(1, &one, NULL));

        // ?: with a null pointer constant in either arm, both orders.
        const char *maybe = 0 ? "a" : (const char *)0;
        printf("%s\n", maybe ? maybe : "null");
        printf("%s\n", (1 ? (char *)0 : "b") ? "non-null" : "null");

        id doc = @{@"users": @[@{@"name": @"ada"}, @{@"name": @"grace"}],
                   @"count": @2};
        printf("%s\n", [dig(doc, @[@"users", @1, @"name"]) UTF8String]);
        printf("%s\n", [dig(doc, @[@"count"]) UTF8String]);
        printf("%s\n", [dig(doc, @[@"nope", @"deeper"]) UTF8String]);

        // And writing through one, which is the setter half of the same
        // sugar on a receiver with no declared class.
        id table = [NSMutableDictionary dictionary];
        table[@"k"] = @"v";
        printf("%s\n", [table[@"k"] UTF8String]);
    }
    return 0;
}
