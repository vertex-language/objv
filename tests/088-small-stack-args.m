// Apple's arm64 ABI packs stack arguments at their natural size: chars and
// shorts past the eighth argument are not widened to 8 bytes each.
#include <stdio.h>

int packed(long r0, long r1, long r2, long r3, long r4, long r5, long r6, long r7,
           char a, short b, char c, int d, char e, long long f, short g) {
    return (int)(r0 + r7) + a + b + c + d + e + (int)f + g;
}

float fpacked(double d0, double d1, double d2, double d3, double d4, double d5,
              double d6, double d7, float a, float b, double c, float d) {
    return (float)(d0 + d7) + a + b * 10 + (float)c * 100 + d * 1000;
}

int main(void) {
    printf("%d\n", packed(1, 0, 0, 0, 0, 0, 0, 2, 3, 400, -5, 60000, 7, 800000, -9));
    printf("%g\n", fpacked(1, 0, 0, 0, 0, 0, 0, 2, 0.5f, 0.25f, 0.125, 0.0625f));
    return 0;
}
