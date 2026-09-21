// A union's members share storage; reading another member reinterprets the
// bytes.
#include <stdio.h>
#include <stdint.h>

union bits { float f; uint32_t u; uint8_t b[4]; };

int main(void) {
    union bits v;
    v.f = 1.0f;
    printf("%08x %zu\n", v.u, sizeof v);
    v.u = 0x40490FDB;
    printf("%.6f\n", v.f);
    v.u = 0x11223344;
    printf("%x %x\n", v.b[0], v.b[3]);
    return 0;
}
