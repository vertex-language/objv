// Bit-fields: width, signedness, packing, and wrapping on store.
#include <stdio.h>

struct flags {
    unsigned a : 1;
    unsigned b : 3;
    signed   c : 4;
    unsigned d : 24;
};

int main(void) {
    struct flags f = { 0 };
    f.a = 1;
    f.b = 9;
    f.c = -3;
    f.d = 0xABCDEF;
    printf("%u %u %d %x %zu\n", f.a, f.b, f.c, f.d, sizeof f);
    f.c = 7;
    f.c++;
    printf("%d\n", f.c);
    return 0;
}
