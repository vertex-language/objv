// Comparisons with NaN are all false except !=.
#include <stdio.h>
#include <math.h>

int main(void) {
    double n = NAN, one = 1.0;
    printf("%d %d %d %d %d %d\n", n < one, n <= one, n > one, n >= one, n == n, n != n);
    printf("%d %d\n", -0.0 == 0.0, 1.5 < 2.5f);
    float f = NAN;
    printf("%d\n", f == f);
    return 0;
}
