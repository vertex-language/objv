// __builtin_*_overflow: the wrapped result and whether it overflowed, which
// is how signed overflow is asked about without undefined behavior.
#include <stdio.h>
#include <limits.h>

int main(void) {
    int r;
    int o = __builtin_add_overflow(INT_MAX, 1, &r);
    printf("%d %d\n", o, r);
    o = __builtin_sub_overflow(INT_MIN, 1, &r);
    printf("%d %d\n", o, r);
    o = __builtin_mul_overflow(65536, 65536, &r);
    printf("%d %d\n", o, r);
    o = __builtin_mul_overflow(1000, 1000, &r);
    printf("%d %d\n", o, r);
    unsigned u;
    o = __builtin_add_overflow(4000000000u, 400000000u, &u);
    printf("%d %u\n", o, u);
    long long ll;
    o = __builtin_mul_overflow(3037000500ll, 3037000500ll, &ll);
    printf("%d\n", o);
    return 0;
}
