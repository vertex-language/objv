// int16_t and uint16_t through parameters and returns.
#include <stdio.h>
#include <stdint.h>

int16_t  neg16(int16_t v)  { return (int16_t)-v; }
uint16_t inv16(uint16_t v) { return (uint16_t)~v; }

int main(void) {
    printf("%d %d\n", neg16(-32768), neg16(1234));
    printf("%u %u\n", inv16(0), inv16(0x1234));
    return 0;
}
