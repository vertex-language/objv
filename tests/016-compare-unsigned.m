// The same bits compared as unsigned: -3 is now the larger.
#include <stdio.h>

int main(void) {
    unsigned a = (unsigned)-3, b = 2;
    printf("%d %d %d %d\n", a < b, a <= b, a > b, a >= b);
    return 0;
}
