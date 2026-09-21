// The six comparisons on signed ints, as values.
#include <stdio.h>

int main(void) {
    int a = -3, b = 2;
    printf("%d %d %d %d %d %d\n", a < b, a <= b, a > b, a >= b, a == b, a != b);
    printf("%d %d\n", b <= b, b >= b);
    return 0;
}
