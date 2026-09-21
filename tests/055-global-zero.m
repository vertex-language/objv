// Globals without initializers are zero, however large.
#include <stdio.h>

int zi;
double zd;
char buf[4096];
struct { int a; long b; char c[8]; } zs;
static int *zp;

int main(void) {
    int nonzero = 0;
    for (int i = 0; i < 4096; i++) nonzero += buf[i] != 0;
    printf("%d %g %d %d %ld %d %d\n", zi, zd, nonzero, zs.a, zs.b, zs.c[7], zp == 0);
    return 0;
}
