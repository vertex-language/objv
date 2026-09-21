// Many values live across calls: they must survive in callee-saved
// registers or the stack, ints and doubles alike.
#include <stdio.h>

__attribute__((noinline)) long clobber(long v) {
    volatile long a = v, b = v * 2, c = v * 3;
    return a + b + c;
}

__attribute__((noinline)) double fclobber(double v) {
    volatile double a = v, b = v * 2;
    return a * b;
}

int main(void) {
    long a = 1, b = 2, c = 3, d = 4, e = 5, f = 6, g = 7, h = 8, i = 9, j = 10, k = 11, l = 12;
    double x = 1.5, y = 2.5, z = 3.5, w = 4.5, u = 5.5, t = 6.5, s = 7.5, r = 8.5, q = 9.5;
    long acc = 0;
    double facc = 0;
    for (int n = 0; n < 4; n++) {
        acc += clobber(n);
        facc += fclobber(n);
        a++; b += a; c += b; d += c; e += d; f += e; g += f; h += g; i += h; j += i; k += j; l += k;
        x += 1; y += x; z += y; w += z; u += w; t += u; s += t; r += s; q += r;
    }
    printf("%ld %ld\n", acc, a + b + c + d + e + f + g + h + i + j + k + l);
    printf("%g %g\n", facc, x + y + z + w + u + t + s + r + q);
    return 0;
}
