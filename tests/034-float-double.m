// float to double is exact; double to float rounds.
#include <stdio.h>

double up(float f) { return f; }
float down(double d) { return (float)d; }

int main(void) {
    printf("%.17g\n", up(0.1f));
    printf("%.9g\n", down(0.1));
    printf("%.9g\n", down(1e40));
    printf("%.9g\n", down(3.4028235e38));
    return 0;
}
