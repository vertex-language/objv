// And, or, xor and not.
#include <stdio.h>

int main(void) {
    unsigned a = 0xF0F0u, b = 0x0FF0u;
    printf("%x %x %x %x\n", a & b, a | b, a ^ b, ~a);
    int n = -1;
    printf("%d %d\n", n & 0x7F, ~n);
    return 0;
}
