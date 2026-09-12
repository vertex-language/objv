// Blocks that outlive the frame that wrote them, and initializers that hold
// a call.
//
// A block literal is an object in the frame that wrote it (§6.9). Under ARC
// the compiler copies it to the heap wherever it escapes -- returned from a
// function, stored into a structure, put in an array -- and under manual
// retain and release the program sends -copy itself. Both spellings are
// here, in both memory models, because the two produce the same program and
// only one of them compiles under each.
//
// The other thing under test is quieter: an aggregate initializer whose
// items are calls, written inside an @autoreleasepool. A call in an
// exception region is an invoke, and an invoke ends its block, so the
// element after it belongs to the invoke's successor. Building the whole
// list against the block the walk started in wrote past the end of a
// terminated block.

#import <Foundation/Foundation.h>

typedef NSInteger (^IntFn)(NSInteger);
typedef NSString *(^Describe)(void);

// The escape hatch both models agree on: under ARC the -copy is a no-op the
// optimizer removes, and under manual retain and release it is the thing
// that makes the return legal.
static IntFn adder(NSInteger by) {
    return [^NSInteger(NSInteger x) { return x + by; } copy];
}

static IntFn multiplier(NSInteger by) {
    IntFn f = ^NSInteger(NSInteger x) { return x * by; };
    return [f copy];
}

// A block stored in a C structure, which is the other way one escapes.
typedef struct {
    IntFn fn;
    const char *name;
} Entry;

// A structure returned by value with a block in it.
static Entry entryFor(NSInteger by, const char *name) {
    Entry e = { adder(by), name };
    return e;
}

static long applyAll(const Entry *t, size_t n, long x) {
    long acc = x;
    for (size_t i = 0; i < n; i++) acc = t[i].fn(acc);
    return acc;
}

int main(void) {
    @autoreleasepool {
        IntFn a5 = adder(5), m3 = multiplier(3);
        printf("%ld %ld\n", (long)a5(10), (long)m3(10));

        // The initializers that ended a block: every item here is a call,
        // and all of them are inside an exception region.
        Entry table[] = {
            { adder(1),      "add1" },
            { multiplier(2), "mul2" },
            { adder(100),    "add100" },
        };
        for (size_t i = 0; i < sizeof table / sizeof table[0]; i++) {
            printf("%s %ld\n", table[i].name, (long)table[i].fn(10));
        }
        printf("%ld\n", applyAll(table, 3, 1));

        // The same shape one level down: a nested brace holding calls.
        struct { Entry a, b; } pair = {
            { adder(7), "seven" },
            { multiplier(7), "times" },
        };
        printf("%ld %ld\n", (long)pair.a.fn(0), (long)pair.b.fn(2));

        // And an array of them, built from a function that returns a
        // structure holding a block.
        Entry built[] = { entryFor(10, "ten"), entryFor(20, "twenty") };
        printf("%s %ld %s %ld\n",
               built[0].name, (long)built[0].fn(1),
               built[1].name, (long)built[1].fn(1));

        // A block that captures another block, which is a capture the
        // runtime has to copy rather than retain.
        IntFn compose = [^NSInteger(NSInteger x) { return a5(m3(x)); } copy];
        printf("%ld\n", (long)compose(4));

        // Recursion through __block, which is the one capture that is a
        // pointer to the frame rather than a copy of what is in it.
        __block IntFn fact = nil;
        fact = ^NSInteger(NSInteger n) { return n <= 1 ? 1 : n * fact(n - 1); };
        printf("%ld\n", (long)fact(6));

        // Blocks in an Objective-C collection, which retains what it holds.
        NSMutableArray *fns = [NSMutableArray array];
        for (NSInteger i = 0; i < 3; i++) [fns addObject:adder(i * 10)];
        for (IntFn f in fns) printf("%ld ", (long)f(0));
        printf("\n");

        // A block returning an object, called through a typedef, inside a
        // structure, inside an initializer with a call in it.
        struct { Describe d; NSInteger tag; } boxed = {
            [^NSString *{ return [NSString stringWithFormat:@"tag-%ld",
                                  (long)a5(0)]; } copy],
            9,
        };
        printf("%s %ld\n", boxed.d().UTF8String, (long)boxed.tag);
    }
    return 0;
}
