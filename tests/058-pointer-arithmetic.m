// Pointer arithmetic scales by the element size.
#include <stdio.h>

int main(void) {
    int a[5] = { 10, 20, 30, 40, 50 };
    int *p = a;
    printf("%d %d %d\n", *(p + 2), p[4], *(a + 1));
    p += 3;
    printf("%d %d\n", *p, p[-1]);
    double d[3] = { 1.5, 2.5, 3.5 };
    double *q = &d[2];
    printf("%.1f %td\n", *--q, q - d);
    long long l[2];
    printf("%td\n", (char *)(l + 1) - (char *)l);
    return 0;
}
