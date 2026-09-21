// Unsigned addition wraps modulo 2^32 (signed overflow is undefined, so it is
// not asked here).
#include <stdio.h>
#include <limits.h>

unsigned add(unsigned a, unsigned b) { return a + b; }

int main(void) {
    printf("%u\n", add(UINT_MAX, 1));
    printf("%u\n", add(UINT_MAX, UINT_MAX));
    printf("%d\n", add(INT_MAX, 1) == 0x80000000u);
    return 0;
}
