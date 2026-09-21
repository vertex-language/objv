// Constants that do not fit in one instruction's immediate: every 16-bit
// piece nonzero, negative, and floating-point constants that are not
// simple.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    uint64_t a = 0x123456789ABCDEF0ull;
    int64_t b = -0x7EDCBA9876543210ll;
    uint32_t c = 0xDEADBEEFu;
    int32_t d = -123456789;
    double e = 1.2345678901234567e-300;
    float f = 3.14159274f;
    printf("%016llx %lld\n", (unsigned long long)a, (long long)b);
    printf("%x %d\n", c, d);
    printf("%.17g %.9g\n", e, f);
    printf("%llu\n", (unsigned long long)(a ^ (uint64_t)b));
    return 0;
}
