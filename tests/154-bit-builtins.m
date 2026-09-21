// Bit-counting builtins: popcount, count leading and trailing zeros, byte
// swap.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    unsigned v[] = { 1, 0x80000000u, 0xF0F0u, 0xFFFFFFFFu, 12 };
    for (int i = 0; i < 5; i++)
        printf("%d %d %d\n", __builtin_popcount(v[i]), __builtin_clz(v[i]), __builtin_ctz(v[i]));
    uint64_t w = 0x0000010000000000ull;
    printf("%d %d %d\n", __builtin_popcountll(w), __builtin_clzll(w), __builtin_ctzll(w));
    printf("%08x %016llx\n", __builtin_bswap32(0x11223344u),
           (unsigned long long)__builtin_bswap64(0x0102030405060708ull));
    printf("%d\n", __builtin_parity(7u));
    return 0;
}
