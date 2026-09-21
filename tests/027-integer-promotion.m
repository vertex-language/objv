// Operands narrower than int are promoted before arithmetic.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    uint8_t a = 200, b = 100;
    int sum = a + b;
    printf("%d\n", sum);
    uint8_t c = 0x80;
    printf("%d\n", c << 1);
    printf("%d\n", (uint8_t)~c);
    printf("%zu\n", sizeof(a + b));
    return 0;
}
