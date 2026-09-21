// Calling through a function pointer, and passing one as an argument.
#include <stdio.h>

int add(int a, int b) { return a + b; }
int mul(int a, int b) { return a * b; }
int apply(int (*f)(int, int), int a, int b) { return f(a, b); }

typedef double (*unary)(double);
double half(double v) { return v / 2; }

int main(void) {
    int (*op)(int, int) = add;
    printf("%d\n", op(3, 4));
    op = &mul;
    printf("%d %d\n", (*op)(3, 4), apply(add, 10, 20));
    unary u = half;
    printf("%g %d\n", u(9), op == mul);
    return 0;
}
