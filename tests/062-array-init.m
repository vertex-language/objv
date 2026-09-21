// Partial and designated array initializers zero the rest.
#include <stdio.h>

int main(void) {
    int a[6] = { 1, 2 };
    int b[6] = { [4] = 9, [1] = 3 };
    char c[] = { 'x', 'y', 'z' };
    for (int i = 0; i < 6; i++) printf("%d/%d ", a[i], b[i]);
    printf("%zu %c\n", sizeof c, c[2]);
    return 0;
}
