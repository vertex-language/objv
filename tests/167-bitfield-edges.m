// Bit-fields: bool fields, a field spanning a storage unit, a zero-width
// field forcing alignment, and a bit-field in a union.
#include <stdio.h>
#include <stdbool.h>

struct s {
    bool flag : 1;
    unsigned a : 7;
    unsigned b : 30;
    unsigned : 0;
    unsigned c : 3;
    long long d : 40;
};

union u { unsigned raw; struct { unsigned lo : 16, hi : 16; } parts; };

int main(void) {
    struct s v = { 0 };
    v.flag = 5;
    v.a = 200;
    v.b = 0x3FFFFFFF;
    v.c = 5;
    v.d = -(1ll << 39);
    printf("%d %u %x %u %lld %zu\n", v.flag, v.a, v.b, v.c, (long long)v.d, sizeof v);
    union u x = { 0x12345678 };
    printf("%x %x\n", x.parts.lo, x.parts.hi);
    x.parts.hi = 0xABCD;
    printf("%x\n", x.raw);
    return 0;
}
