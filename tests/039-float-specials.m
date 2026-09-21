// Infinity, negative zero, and NaN, made by arithmetic.
#include <stdio.h>
#include <math.h>

double div(double a, double b) { return a / b; }

int main(void) {
    printf("%g %g\n", div(1, 0), div(-1, 0));
    printf("%g\n", div(-1, INFINITY));
    printf("%d\n", isnan(div(0, 0)));
    printf("%d\n", signbit(-0.0) != 0);
    printf("%g\n", 1e308 * 10);
    return 0;
}
