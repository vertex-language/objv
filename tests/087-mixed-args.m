// Ints and floats interleaved, both classes spilling to the stack.
#include <stdio.h>

double mix(int a, double b, int c, double d, int e, double f, int g, double h,
           int i, double j, int k, double l, int m, double n, int o, double p,
           int q, double r, int s, double t) {
    return a + b + c + d + e + f + g + h + i + j + k + l + m + n + o + p
         + q * 100 + r * 1000 + s * 10000 + t * 100000;
}

int main(void) {
    printf("%.1f\n", mix(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
                         17, 18.5, 19, 20.25));
    return 0;
}
