// Unsigned division and remainder: the same bits as a negative int, a
// different answer.
#include <stdio.h>

unsigned uquo(unsigned a, unsigned b) { return a / b; }
unsigned urem(unsigned a, unsigned b) { return a % b; }

int main(void) {
    printf("%u %u\n", uquo(0xFFFFFFFEu, 2), urem(0xFFFFFFFFu, 10));
    printf("%u %u\n", uquo((unsigned)-7, 2), urem((unsigned)-7, 2));
    return 0;
}
