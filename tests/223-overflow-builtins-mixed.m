// Checked arithmetic across types: operands of different signedness and
// width, results narrower than the operands, and the edges -- INT64_MIN * -1,
// unsigned subtraction going below zero, a sum that fits only because the
// result type is signed. The stored value is always the truncated exact one.
#include <stdio.h>
#include <stdint.h>
#include <limits.h>

int main(void) {
    int8_t c;
    printf("%d %d\n", __builtin_add_overflow(100, 27, &c), c);
    printf("%d %d\n", __builtin_add_overflow(100, 28, &c), c);
    printf("%d %d\n", __builtin_sub_overflow(-100, 29, &c), c);
    uint8_t uc;
    printf("%d %u\n", __builtin_mul_overflow(16, 16, &uc), uc);
    printf("%d %u\n", __builtin_sub_overflow(3u, 5, &uc), uc);
    short s;
    printf("%d %d\n", __builtin_mul_overflow(-181, 181, &s), s);
    unsigned u;
    printf("%d %u\n", __builtin_sub_overflow(2u, 3u, &u), u);
    printf("%d %u\n", __builtin_add_overflow(-1, 1u, &u), u);
    int i;
    printf("%d %d\n", __builtin_add_overflow(3000000000u, -1000000000, &i), i);
    printf("%d %d\n", __builtin_mul_overflow((char)-128, (unsigned char)255, &i), i);
    int64_t l;
    printf("%d %lld\n", __builtin_mul_overflow(INT64_MIN, -1ll, &l), (long long)l);
    printf("%d %lld\n", __builtin_sub_overflow(INT64_MIN, 1ll, &l), (long long)l);
    uint64_t ul;
    printf("%d %llu\n", __builtin_mul_overflow(UINT32_MAX, UINT32_MAX, &ul), (unsigned long long)ul);
    printf("%d %llu\n", __builtin_add_overflow(-5, 10, &ul), (unsigned long long)ul);
    printf("%d %llu\n", __builtin_add_overflow(-5, 4, &ul), (unsigned long long)ul);
    // 65 bits between the operands and the result.
    printf("%d %lld\n", __builtin_add_overflow(INT64_MIN, UINT64_MAX, &l), (long long)l);
    printf("%d %lld\n", __builtin_add_overflow((int64_t)-1, (uint64_t)1 << 63, &l), (long long)l);
    printf("%d %llu\n", __builtin_sub_overflow((uint64_t)5, (int64_t)-7, &ul), (unsigned long long)ul);
    printf("%d %llu\n", __builtin_sub_overflow(UINT64_MAX, (int64_t)-1, &ul), (unsigned long long)ul);
    printf("%d %llu\n", __builtin_mul_overflow(UINT64_MAX, UINT64_MAX, &ul), (unsigned long long)ul);
    printf("%d %lld\n", __builtin_mul_overflow(INT64_MIN, (uint64_t)1, &l), (long long)l);
    printf("%d %lld\n", __builtin_mul_overflow((int64_t)-2, (uint64_t)1 << 62, &l), (long long)l);
    printf("%d %llu\n", __builtin_mul_overflow((int64_t)-1, (int64_t)-1, &ul), (unsigned long long)ul);
    return 0;
}
