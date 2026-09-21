// Structs mixing float and integer members are not HFAs: they travel in
// integer registers or memory.
#include <stdio.h>

typedef struct { float f; int i; } FI;
typedef struct { double d; long l; } DL;
typedef struct { char c; double d; float f; } CDF;

FI fi(int v) { return (FI){ v * 0.5f, v }; }
DL dl(DL x) { x.d *= 2; x.l *= 3; return x; }
CDF cdf(CDF x) { x.c++; x.d = -x.d; x.f += 1; return x; }

int main(void) {
    FI a = fi(9);
    DL b = dl((DL){ 1.25, 7 });
    CDF c = cdf((CDF){ 'a', 2.5, 0.5f });
    printf("%g %d | %g %ld | %c %g %g\n", a.f, a.i, b.d, b.l, c.c, c.d, c.f);
    return 0;
}
