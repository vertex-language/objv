// More integer arguments than registers: the ninth and later go on the
// stack.
#include <stdio.h>

long many(long a, long b, long c, long d, long e, long f, long g, long h,
          long i, long j, long k) {
    return a + 2 * b + 3 * c + 4 * d + 5 * e + 6 * f + 7 * g + 8 * h
         + 9 * i + 10 * j + 11 * k;
}

int main(void) {
    printf("%ld\n", many(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11));
    return 0;
}
