// Left shifts, by constants and by a variable count.
#include <stdio.h>

unsigned shl(unsigned x, int n) { return x << n; }

int main(void) {
    printf("%u %u %u\n", shl(1, 0), shl(1, 31), shl(0xFFu, 28));
    unsigned long long big = 1ull << 40;
    printf("%llu\n", big);
    return 0;
}
