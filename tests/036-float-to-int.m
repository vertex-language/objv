// Floating point to integer truncates toward zero (only values in range; the
// rest is undefined).
#include <stdio.h>

int main(void) {
    double v[] = { 2.9, -2.9, 0.5, -0.5, 2147483647.0, -2147483648.0 };
    for (int i = 0; i < 6; i++)
        printf("%d\n", (int)v[i]);
    float f = 255.99f;
    printf("%u\n", (unsigned char)f);
    printf("%lld\n", (long long)-1e15);
    return 0;
}
