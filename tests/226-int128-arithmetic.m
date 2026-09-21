// 128-bit arithmetic, every operator at its edges: carries and borrows
// across the words, the full product, signed and unsigned division and
// remainder, comparisons decided by the high word and by the low one, and
// every shift count from 0 to 127 in both directions, folded to a checksum.
#include <stdio.h>
#include <stdint.h>

typedef unsigned __int128 u128;
typedef __int128 s128;

static void show(const char *tag, u128 v) {
    printf("%s %016llx%016llx\n", tag, (unsigned long long)(v >> 64), (unsigned long long)v);
}

int main(void) {
    u128 max = ~(u128)0;
    u128 carry = (u128)UINT64_MAX + 1;
    show("carry", carry);
    show("borrow", carry - 1);
    show("wrap", max + 1);
    show("neg", -(u128)1);
    s128 m = -((s128)1 << 100);
    show("prod", (u128)0x123456789abcdef0ull * 0xfedcba9876543210ull);
    show("sprod", (u128)(m * -3));
    show("udiv", max / 7);
    show("umod", max % 1000000007u);
    show("sdiv", (u128)(m / 12345));
    show("smod", (u128)(m % 12345));
    show("and", max & ((u128)0xf0 << 60));
    show("or", carry | 5);
    show("xor", max ^ carry);
    show("not", ~carry);
    s128 a = -5, b = 3;
    u128 ua = (u128)-5, ub = 3;
    printf("%d %d %d %d %d %d\n", a < b, a > b, a <= b, a >= b, a == b, a != b);
    printf("%d %d %d %d\n", ua < ub, ua > ub, (u128)1 << 64 > UINT64_MAX, carry == (carry - 1) + 1);
    printf("%d %d\n", (s128)1 << 64 < (s128)((u128)1 << 64) + 1, (s128)-1 < (s128)0);
    u128 sum = 0;
    s128 ssum = 0;
    u128 pattern = ((u128)0x8123456789abcdefull << 64) | 0xfedcba9876543210ull;
    for (int n = 0; n < 128; n++) {
        sum += (pattern << n) ^ (pattern >> n) ^ n;
        ssum += ((s128)pattern >> n) + n;
    }
    show("shl/shr", sum);
    show("sar", (u128)ssum);
    int count = 70;
    show("var", pattern << count);
    printf("%d\n", !carry + !(carry - carry));
    return 0;
}
