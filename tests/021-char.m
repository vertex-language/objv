// Plain char is signed on arm64 Darwin; signed and unsigned char differ above
// 127.
#include <stdio.h>

int main(void) {
    char c = (char)200;
    signed char s = (signed char)200;
    unsigned char u = 200;
    printf("%d %d %d\n", c, s, u);
    printf("%d %d\n", c < 0, u > 127);
    return 0;
}
