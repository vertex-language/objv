// A struct holding a float array is still a homogeneous aggregate; one with
// five members is not, and goes by reference.
#include <stdio.h>

typedef struct { float v[4]; } F4;
typedef struct { double v[2]; } D2;
typedef struct { float v[5]; } F5;

F4 scale4(F4 a, float k) { for (int i = 0; i < 4; i++) a.v[i] *= k; return a; }
D2 swap2(D2 a) { return (D2){ { a.v[1], a.v[0] } }; }
F5 rev5(F5 a) { F5 r; for (int i = 0; i < 5; i++) r.v[i] = a.v[4 - i]; return r; }

int main(void) {
    F4 a = scale4((F4){ { 1, 2, 3, 4 } }, 0.5f);
    D2 b = swap2((D2){ { 1.25, -8 } });
    F5 c = rev5((F5){ { 1, 2, 3, 4, 5 } });
    printf("%g %g %g %g\n", a.v[0], a.v[1], a.v[2], a.v[3]);
    printf("%g %g\n", b.v[0], b.v[1]);
    printf("%g %g %g\n", c.v[0], c.v[2], c.v[4]);
    return 0;
}
