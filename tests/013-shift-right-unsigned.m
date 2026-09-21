// Right shift of an unsigned value brings in zeros.
#include <stdio.h>

unsigned shr(unsigned x, int n) { return x >> n; }

int main(void) {
    printf("%u %u\n", shr(0x80000000u, 31), shr(0xFFFFFFFFu, 4));
    unsigned char c = 0xF0;
    printf("%d\n", c >> 4);
    return 0;
}
