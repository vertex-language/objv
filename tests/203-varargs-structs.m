// Structs of every size class in a var-tail: small, 12 bytes, 16 bytes,
// over 16 (passed by reference), and floats-only, through a C function and
// a variadic method.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>
#include <stdarg.h>

typedef struct { char a, b, c; } C3;
typedef struct { int a, b, c; } I3;
typedef struct { long a; double b; } LD;
typedef struct { long v[3]; } L3;
typedef struct { float x, y, z; } F3;

static void show(const char *kinds, va_list ap) {
    for (const char *k = kinds; *k; k++) {
        switch (*k) {
        case 'c': { C3 v = va_arg(ap, C3); printf("c3 %c%c%c\n", v.a, v.b, v.c); break; }
        case 'i': { I3 v = va_arg(ap, I3); printf("i3 %d %d %d\n", v.a, v.b, v.c); break; }
        case 'l': { LD v = va_arg(ap, LD); printf("ld %ld %g\n", v.a, v.b); break; }
        case 'b': { L3 v = va_arg(ap, L3); printf("l3 %ld %ld %ld\n", v.v[0], v.v[1], v.v[2]); break; }
        case 'f': { F3 v = va_arg(ap, F3); printf("f3 %g %g %g\n", v.x, v.y, v.z); break; }
        case 'd': printf("d %g\n", va_arg(ap, double)); break;
        }
    }
}

static void fn(const char *kinds, ...) {
    va_list ap;
    va_start(ap, kinds);
    show(kinds, ap);
    va_end(ap);
}

@interface Printer : NSObject
- (void)print:(const char *)kinds, ...;
@end
@implementation Printer
- (void)print:(const char *)kinds, ... {
    va_list ap;
    va_start(ap, kinds);
    show(kinds, ap);
    va_end(ap);
}
@end

int main(void) {
    C3 c = { 'x', 'y', 'z' };
    I3 i = { 1, -2, 3 };
    LD l = { 1ll << 40, 0.5 };
    L3 b = { { 7, 8, 9 } };
    F3 f = { 1.5f, 2.5f, -3 };
    fn("cdilbfd", c, 1.25, i, l, b, f, -4.0);
    [[Printer new] print:"fcl", f, c, l];
    return 0;
}
