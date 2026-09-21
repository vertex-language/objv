// for loops: counting up, counting down, and a declaration in the header.
#include <stdio.h>

int main(void) {
    int sum = 0;
    for (int i = 1; i <= 100; i++) sum += i;
    printf("%d\n", sum);
    for (int i = 3; i > 0; i--) printf("%d ", i);
    printf("\n");
    int j;
    for (j = 0; j * j < 50; j++) {}
    printf("%d\n", j);
    return 0;
}
