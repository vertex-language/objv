// short and unsigned short, and a store that wraps into 16 bits.
#include <stdio.h>

short sh(int v) { return (short)v; }
unsigned short ush(int v) { return (unsigned short)v; }

int main(void) {
    printf("%d %d %d\n", sh(32767), sh(32768), sh(-32769));
    printf("%u %u\n", ush(65535), ush(65536 + 3));
    short a = 30000, b = 30000;
    int sum = a + b;
    printf("%d\n", sum);
    return 0;
}
