// Integers to floating point, including values float cannot hold exactly.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    int32_t i = 16777217;
    printf("%.9g %.17g\n", (float)i, (double)i);
    int64_t big = 9007199254740993ll;
    printf("%.17g\n", (double)big);
    printf("%.9g\n", (float)-7);
    unsigned char c = 255;
    printf("%.3f\n", (double)c);
    return 0;
}
