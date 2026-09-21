// The edges of int32_t and uint32_t.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    int32_t lo = INT32_MIN, hi = INT32_MAX;
    uint32_t u = UINT32_MAX;
    printf("%d %d %u\n", lo, hi, u);
    printf("%d %u\n", lo + hi, u - 1);
    printf("%d\n", -(lo + 1));
    return 0;
}
