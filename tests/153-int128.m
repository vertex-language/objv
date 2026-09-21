// __int128: a 128-bit product, shifts, and division, printed as two halves.
#include <stdio.h>
#include <stdint.h>

typedef unsigned __int128 u128;

static void show(u128 v) {
    printf("%016llx%016llx\n", (unsigned long long)(v >> 64), (unsigned long long)v);
}

u128 mul(uint64_t a, uint64_t b) { return (u128)a * b; }

int main(void) {
    show(mul(UINT64_MAX, UINT64_MAX));
    u128 x = (u128)1 << 100;
    show(x);
    show(x / 3);
    show(x % 1000000007u);
    __int128 n = -((__int128)1 << 70);
    printf("%lld\n", (long long)(n >> 64));
    printf("%zu\n", sizeof(u128));
    return 0;
}
