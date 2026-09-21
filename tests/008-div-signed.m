// Signed division truncates toward zero.
#include <stdio.h>

int quo(int a, int b) { return a / b; }

int main(void) {
    printf("%d %d %d %d\n", quo(7, 2), quo(-7, 2), quo(7, -2), quo(-7, -2));
    printf("%d\n", quo(-2147483647 - 1, 1));
    return 0;
}
