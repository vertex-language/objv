// A compound assignment computes in the common type of both sides and
// converts the result back: `x *= 1.5` multiplies as double, a narrow type
// wraps, a _Bool stays 0 or 1. ++ and -- do the same.
#include <stdio.h>
#include <stdbool.h>

int main(void) {
    int x = 3;
    x *= 1.5;
    printf("%d\n", x);
    unsigned char c = 200;
    c += 100;
    printf("%d\n", c);
    short s = 1;
    s <<= 20;
    printf("%d\n", s);
    bool b = true;
    b += 1;
    printf("%d\n", b);
    int n = -7;
    n /= 2u;
    printf("%d\n", n);
    float f = 1;
    f += 1e-8;
    printf("%.9g\n", f);
    signed char sc = -128;
    sc--;
    unsigned short us = 0;
    us--;
    printf("%d %u\n", sc, us);
    bool t = false;
    t++;
    t++;
    printf("%d\n", t);
    return 0;
}
