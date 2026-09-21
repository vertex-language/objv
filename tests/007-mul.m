// Multiplication, including the unsigned high bits that are thrown away.
#include <stdio.h>

int mul(int a, int b) { return a * b; }
unsigned umul(unsigned a, unsigned b) { return a * b; }

int main(void) {
    printf("%d %d %d\n", mul(6, 7), mul(-6, 7), mul(-6, -7));
    printf("%u\n", umul(0x10000u, 0x10001u));
    return 0;
}
