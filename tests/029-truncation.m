// Narrowing conversions keep the low bits.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    int64_t big = 0x123456789ABCDEFll;
    printf("%x\n", (uint32_t)big);
    printf("%x\n", (uint16_t)big);
    printf("%d\n", (int8_t)big);
    printf("%d\n", (int16_t)0x18000);
    return 0;
}
