// NSValue wraps structs; @() boxes a struct marked objc_boxable.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct __attribute__((objc_boxable)) { int a; double b; } Pair;

int main(void) {
    @autoreleasepool {
        NSValue *r = [NSValue valueWithRange:NSMakeRange(3, 4)];
        NSRange back = r.rangeValue;
        printf("%lu %lu\n", (unsigned long)back.location, (unsigned long)back.length);
        NSValue *pt = [NSValue valueWithPoint:NSMakePoint(1.5, -2)];
        printf("%g %g\n", pt.pointValue.x, pt.pointValue.y);
        Pair p = { 7, 0.25 };
        NSValue *boxed = @(p);
        Pair q;
        [boxed getValue:&q size:sizeof q];
        printf("%d %g %s\n", q.a, q.b, boxed.objCType);
        NSArray *pts = @[ [NSValue valueWithRange:NSMakeRange(1, 1)], [NSValue valueWithRange:NSMakeRange(2, 2)] ];
        printf("%lu\n", (unsigned long)[pts[1] rangeValue].length);
    }
    return 0;
}
