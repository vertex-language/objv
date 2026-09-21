// uint64 to double and back, above INT64_MAX where a signed conversion
// would be wrong.
#include <stdio.h>
#include <stdint.h>

double   u2d(uint64_t v) { return (double)v; }
uint64_t d2u(double v)   { return (uint64_t)v; }
float    u2f(uint32_t v) { return (float)v; }

int main(void) {
    printf("%.17g\n", u2d(UINT64_MAX));
    printf("%.17g\n", u2d(0x8000000000000000ull));
    printf("%llu\n", (unsigned long long)d2u(1.8e19));
    printf("%.9g\n", u2f(4000000000u));
    return 0;
}
