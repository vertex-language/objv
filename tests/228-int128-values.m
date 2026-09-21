// 128-bit integers as values: parameters in every register position -- after
// one int, which on Apple's arm64 is x1/x2 with no rounding to an even
// register -- and on the stack once fewer than two registers are left,
// alongside doubles, returned, called through a pointer, and as struct
// members, array elements, globals and statics aligned to 16.
#include <stdio.h>
#include <stdint.h>
#include <stddef.h>

typedef unsigned __int128 u128;
typedef __int128 s128;

static void show(const char *tag, u128 v) {
    printf("%s %016llx%016llx\n", tag, (unsigned long long)(v >> 64), (unsigned long long)v);
}

__attribute__((noinline)) static u128 odd(int a, u128 b, int c) { return b * (unsigned)a + (unsigned)c; }
__attribute__((noinline)) static s128 late(long a, long b, long c, long d, long e, long f, long g,
                                          s128 h, double x, s128 i) {
    return h + i + a + b + c + d + e + f + g + (s128)x;
}
__attribute__((noinline)) static u128 many(u128 a, u128 b, u128 c, u128 d, u128 e) {
    return a + (b << 1) + (c << 2) + (d << 3) + (e << 4);
}

struct rec { char tag; u128 value; short after; };
static u128 table[3] = { 1, (u128)1 << 64, 0 };
s128 global = -7;

int main(void) {
    show("odd", odd(3, ((u128)5 << 64) | 9, 4));
    show("late", (u128)late(1, 2, 3, 4, 5, 6, 7, (s128)1 << 80, 2.5, -((s128)1 << 70)));
    show("many", many(1, 2, 3, (u128)1 << 64, (u128)1 << 120));
    u128 (*fp)(int, u128, int) = odd;
    show("pointer", fp(2, 100, 1));
    struct rec r = { 'x', ((u128)0xabc << 64) | 0xdef, -2 };
    printf("%zu %zu %zu %zu\n", sizeof(struct rec), offsetof(struct rec, value), offsetof(struct rec, after),
           _Alignof(u128));
    show("member", r.value);
    table[2] = table[0] + table[1];
    show("table", table[2]);
    static s128 counter = 5;
    counter += global;
    show("static", (u128)counter);
    printf("%d %d\n", (int)((uintptr_t)&table[1] % 16), (int)((uintptr_t)&r.value % 16));
    return 0;
}
