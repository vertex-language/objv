// Subtraction, signed and unsigned, including below zero.
#include <stdio.h>

int sub(int a, int b) { return a - b; }
unsigned usub(unsigned a, unsigned b) { return a - b; }

int main(void) {
    printf("%d %d %d\n", sub(10, 3), sub(3, 10), sub(-5, -5));
    printf("%u\n", usub(0, 1));
    return 0;
}
