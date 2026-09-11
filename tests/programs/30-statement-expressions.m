// The macro that evaluates its argument once.
//
// A statement expression is the only way to write one. `#define MAX(a, b)
// ((a) > (b) ? (a) : (b))` evaluates both arguments twice, so `MAX(i++, j)`
// increments twice and the bug is in every use of the macro rather than in
// the macro. `({ __typeof__(a) _a = (a); … })` names the value first, which
// is why <sys/param.h>, <AssertMacros.h> and most of Apple's defensive
// macros are written this way.
//
// What it exercises here is that the construct is an ordinary block wearing
// an expression's clothes: it declares variables, it loops, it can hold a
// @try, it has its own ARC scope, and its value is the last expression
// statement's — of whatever type that is, aggregates included.

#import <Foundation/Foundation.h>
#include <string.h>

#define MAX(a, b) ({ __typeof__(a) _a = (a); __typeof__(b) _b = (b); _a > _b ? _a : _b; })
#define MIN(a, b) ({ __typeof__(a) _a = (a); __typeof__(b) _b = (b); _a < _b ? _a : _b; })
#define CLAMP(x, lo, hi) MIN(MAX((x), (lo)), (hi))
#define SWAP(a, b) do { __typeof__(a) _t = (a); (a) = (b); (b) = _t; } while (0)

// The value of a statement expression may be a structure, which travels the
// same way any other structure of its size does.
#define MIDPOINT(a, b) ({                    \
    CGPoint _p = (a), _q = (b);              \
    CGPoint _m = { (_p.x + _q.x) / 2,        \
                   (_p.y + _q.y) / 2 };      \
    _m;                                      \
})

// Under ARC the block is a scope like any other: what it retains inside, it
// releases at the end, and what it yields survives to the enclosing
// full expression.
#define FIRST_OR(a, d) ({ NSArray *_a = (a); _a.count ? _a.firstObject : (d); })

// A statement expression is an expression, so it nests inside another one
// and inside its own arguments.
#define SUM_TO(n) ({ int _t = 0; for (int _i = 1; _i <= (n); _i++) _t += _i; _t; })

@interface Ledger : NSObject
@property (nonatomic) NSInteger balance;
- (NSInteger)applyDelta:(NSInteger)d;
@end

@implementation Ledger
- (NSInteger)applyDelta:(NSInteger)d {
    // A @try inside a statement expression: the block has its own exception
    // region and still produces a value.
    return ({
        NSInteger before = self.balance;
        @try {
            if (before + d < 0) {
                @throw [NSException exceptionWithName:@"Overdraft"
                                               reason:@"balance would go negative"
                                             userInfo:nil];
            }
            self.balance = before + d;
        } @catch (NSException *e) {
            printf("caught: %s\n", e.name.UTF8String);
        } @finally {
            printf("applied: %ld -> %ld\n", (long)before, (long)self.balance);
        }
        self.balance;
    });
}
@end

static int sideEffects;
static int tick(void) { return ++sideEffects; }

int main(void) {
    @autoreleasepool {
        int i = 3, j = 5;
        printf("%d %d\n", MAX(i, j), MIN(i, j));

        // Once, not twice. This is the whole point of the construct.
        printf("%d %d\n", MAX(tick(), 0), sideEffects);

        printf("%d %d %d\n", CLAMP(1, 3, 9), CLAMP(5, 3, 9), CLAMP(20, 3, 9));

        SWAP(i, j);
        printf("%d %d\n", i, j);

        printf("%g %g\n", MAX(1.5, 2.25), MIN(-1.5, 2.25));

        CGPoint m = MIDPOINT(CGPointMake(0, 0), CGPointMake(4, 10));
        printf("%g %g\n", m.x, m.y);

        printf("%d %d\n", SUM_TO(10), SUM_TO(SUM_TO(3)));

        NSArray *full = @[@"alpha", @"beta"];
        NSArray *empty = @[];
        printf("%s %s\n",
               [FIRST_OR(full, @"(none)") UTF8String],
               [FIRST_OR(empty, @"(none)") UTF8String]);

        // A block that builds an object and hands back a copy: everything it
        // made along the way is gone by the semicolon, and the value is not.
        NSString *joined = ({
            NSMutableString *acc = [NSMutableString string];
            for (NSString *s in full) {
                if (acc.length) [acc appendString:@"-"];
                [acc appendString:s];
            }
            [acc copy];
        });
        printf("%s %zu\n", joined.UTF8String, strlen(joined.UTF8String));

        // A void-valued one, which is a statement that happens to be
        // spelled as an expression.
        ({ printf("void arm\n"); });

        // A declaration inside it shadows, and the shadow ends with it.
        int shadowed = 1;
        int outer = ({ int shadowed = 2; shadowed * 10; });
        printf("%d %d\n", shadowed, outer);

        Ledger *l = [[Ledger alloc] init];
        l.balance = 100;
        printf("%ld\n", (long)[l applyDelta:-40]);
        printf("%ld\n", (long)[l applyDelta:-1000]);
    }
    return 0;
}
