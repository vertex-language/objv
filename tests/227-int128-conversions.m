// 128-bit integers converted to and from every other scalar: each integer
// width with the sign extension or truncation its type says, _Bool, float and
// double both ways through compiler-rt, and pointers; and changed in place
// with ++, --, compound assignment and shifts.
#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>

typedef unsigned __int128 u128;
typedef __int128 s128;

static void show(const char *tag, u128 v) {
    printf("%s %016llx%016llx\n", tag, (unsigned long long)(v >> 64), (unsigned long long)v);
}

int main(void) {
    int8_t c = -3;
    uint32_t u = 0xfffffff0u;
    int64_t l = INT64_MIN;
    show("from int8", (s128)c);
    show("from uint32", (u128)u);
    show("from int64", (s128)l);
    show("from uint64 as signed", (s128)UINT64_MAX);
    s128 big = ((s128)0x7fffffff << 64) | 0x80000001;
    printf("%d %u %lld %d %d\n", (int)big, (unsigned short)big, (long long)big, (int8_t)big, (bool)big);
    printf("%d %d\n", (bool)((u128)1 << 127), (bool)(s128)0);
    double d = 1e30;
    show("from double", (u128)d);
    show("from negative double", (u128)(s128)-12345.75);
    show("from float", (u128)(float)3.5e20f);
    printf("%.17g %.17g\n", (double)((u128)1 << 100), (double)(-((s128)1 << 90) - 1));
    printf("%.9g\n", (float)(((u128)1 << 70) + 12345));
    int x = 42;
    int *p = &x;
    u128 addr = (u128)(uintptr_t)p;
    printf("%d\n", *(int *)(uintptr_t)addr);
    u128 v = UINT64_MAX;
    v++;
    show("inc", v);
    v--;
    v--;
    show("dec", v);
    s128 s = -1;
    s += 10;
    s *= -(s128)1000000000000ll;
    s /= 7;
    s %= (s128)1 << 40;
    s <<= 5;
    s >>= 2;
    s ^= 0x55;
    show("compound", (u128)s);
    s128 before = s++;
    show("post", (u128)(before - s));
    return 0;
}
