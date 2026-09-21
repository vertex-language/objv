// Blocks taking and returning structs: small, HFA, and large.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { float x, y; } V2;
typedef struct { long v[5]; } Big;

int main(void) {
    V2 (^add)(V2, V2) = ^V2(V2 a, V2 b) { return (V2){ a.x + b.x, a.y + b.y }; };
    V2 r = add((V2){ 1, 2 }, (V2){ 0.5f, -3 });
    printf("%g %g\n", r.x, r.y);
    long base = 100;
    Big (^make)(int) = ^Big(int n) { Big b; for (int i = 0; i < 5; i++) b.v[i] = base + n * i; return b; };
    Big b = make(3);
    printf("%ld %ld\n", b.v[0], b.v[4]);
    NSRange (^rng)(NSRange) = ^NSRange(NSRange x) { return NSMakeRange(x.location + 1, x.length * 2); };
    NSRange n = rng(NSMakeRange(4, 5));
    printf("%lu %lu\n", (unsigned long)n.location, (unsigned long)n.length);
    return 0;
}
