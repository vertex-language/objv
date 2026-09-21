// int8_t and uint8_t arithmetic, stored back through the narrow type.
#include <stdio.h>
#include <stdint.h>

int8_t  add8(int8_t a, int8_t b)   { return (int8_t)(a + b); }
uint8_t addu8(uint8_t a, uint8_t b) { return (uint8_t)(a + b); }

int main(void) {
    printf("%d %d\n", add8(100, 27), add8(100, 28));
    printf("%u %u\n", addu8(200, 55), addu8(200, 56));
    return 0;
}
