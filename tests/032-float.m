// float arithmetic, printed with enough digits to show every bit.
#include <stdio.h>

float fadd(float a, float b) { return a + b; }
float fmul(float a, float b) { return a * b; }
float fdiv(float a, float b) { return a / b; }

int main(void) {
    printf("%.9g %.9g %.9g\n", fadd(0.1f, 0.2f), fmul(1.1f, 3.0f), fdiv(1.0f, 3.0f));
    printf("%.9g\n", -fadd(2.5f, 0.25f));
    return 0;
}
