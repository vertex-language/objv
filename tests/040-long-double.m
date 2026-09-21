// long double is double on arm64 Darwin: same size, same answers.
#include <stdio.h>

long double half(long double v) { return v / 2; }

int main(void) {
    long double x = 1.0L / 3.0L;
    printf("%zu\n", sizeof(long double));
    printf("%.17Lg\n", x);
    printf("%.17Lg\n", half(7.0L));
    printf("%.17g\n", (double)(x * 3));
    return 0;
}
