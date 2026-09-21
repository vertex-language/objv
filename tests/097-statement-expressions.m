// GNU statement expressions: a block whose last statement is its value.
#include <stdio.h>

#define MAX(a, b) ({ __typeof__(a) _a = (a); __typeof__(b) _b = (b); _a > _b ? _a : _b; })

int main(void) {
    int i = 3;
    int m = MAX(i++, 2);
    printf("%d %d\n", m, i);
    int v = ({ int t = 0; for (int k = 0; k < 5; k++) t += k; t; });
    printf("%d\n", v);
    double d = MAX(1.5, 2.25);
    printf("%g\n", d);
    return 0;
}
