// The remainder takes the sign of the dividend.
#include <stdio.h>

int rem(int a, int b) { return a % b; }

int main(void) {
    printf("%d %d %d %d\n", rem(7, 3), rem(-7, 3), rem(7, -3), rem(-7, -3));
    return 0;
}
