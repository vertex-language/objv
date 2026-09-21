// 64-bit signed and unsigned arithmetic, division included.
#include <stdio.h>
#include <stdint.h>

int64_t  mul64(int64_t a, int64_t b)   { return a * b; }
uint64_t div64(uint64_t a, uint64_t b) { return a / b; }

int main(void) {
    printf("%lld\n", (long long)mul64(3000000000ll, -3));
    printf("%llu\n", (unsigned long long)div64(UINT64_MAX, 7));
    printf("%lld %llu\n", (long long)INT64_MIN, (unsigned long long)UINT64_MAX);
    printf("%lld\n", (long long)(-9000000000ll % 7));
    return 0;
}
