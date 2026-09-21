// More floating-point arguments than the eight float registers.
#include <stdio.h>

double many(double a, double b, double c, double d, double e, double f,
            double g, double h, double i, double j, float k) {
    return a + b * 2 + c * 3 + d * 4 + e * 5 + f * 6 + g * 7 + h * 8
         + i * 9 + j * 10 + k * 11;
}

int main(void) {
    printf("%g\n", many(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 0.5f));
    return 0;
}
