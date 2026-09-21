// do-while runs its body once before testing.
#include <stdio.h>

int main(void) {
    int i = 10;
    do { printf("%d\n", i); i++; } while (i < 3);
    int digits = 0, v = 12345;
    do { digits++; v /= 10; } while (v);
    printf("%d\n", digits);
    return 0;
}
