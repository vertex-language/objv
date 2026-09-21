// A variadic function reading ints: on arm64 Darwin every variadic argument
// is on the stack.
#include <stdio.h>
#include <stdarg.h>

int sum(int n, ...) {
    va_list ap;
    va_start(ap, n);
    int s = 0;
    for (int i = 0; i < n; i++) s += va_arg(ap, int);
    va_end(ap);
    return s;
}

int main(void) {
    printf("%d %d %d\n", sum(0), sum(3, 1, 2, 3), sum(10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10));
    return 0;
}
