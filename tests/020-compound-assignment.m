// Every compound assignment operator, each on the result of the last.
#include <stdio.h>

int main(void) {
    int x = 100;
    x += 5;  printf("%d\n", x);
    x -= 10; printf("%d\n", x);
    x *= 3;  printf("%d\n", x);
    x /= 4;  printf("%d\n", x);
    x %= 50; printf("%d\n", x);
    x <<= 3; printf("%d\n", x);
    x >>= 1; printf("%d\n", x);
    x &= 0x3C; printf("%d\n", x);
    x |= 0x101; printf("%d\n", x);
    x ^= 0xFF; printf("%d\n", x);
    return 0;
}
