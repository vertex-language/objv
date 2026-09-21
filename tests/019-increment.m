// Prefix and postfix ++ and --, and unary minus.
#include <stdio.h>

int main(void) {
    int i = 5;
    int a = i++;
    int b = ++i;
    int c = i--;
    int d = --i;
    printf("%d %d %d %d %d %d\n", a, b, c, d, i, -i);
    return 0;
}
