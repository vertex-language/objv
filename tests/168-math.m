// <math.h>: functions whose results are exact enough to print, float and
// double.
// libraries: m
#include <stdio.h>
#include <math.h>

int main(void) {
    printf("%.17g %.17g %.17g\n", sqrt(2.0), floor(-2.5), ceil(-2.5));
    printf("%.17g %.17g %.17g\n", fmod(10.5, 3), pow(2, 10), fabs(-3.25));
    printf("%.9g %.9g\n", sqrtf(3.0f), roundf(2.5f));
    printf("%.17g %.17g\n", trunc(-7.9), fmax(1, nan("")));
    printf("%ld %.17g\n", lround(-3.5), ldexp(1.5, 4));
    return 0;
}
