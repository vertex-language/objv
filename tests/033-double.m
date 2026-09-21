// double arithmetic.
#include <stdio.h>

double dsub(double a, double b) { return a - b; }
double ddiv(double a, double b) { return a / b; }

int main(void) {
    printf("%.17g %.17g\n", 0.1 + 0.2, dsub(1.0, 0.9));
    printf("%.17g %.17g\n", ddiv(2.0, 3.0), 1e308 * 10.0 / 1e308);
    return 0;
}
