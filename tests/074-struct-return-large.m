// Returning a struct over 16 bytes goes through memory the caller provides.
#include <stdio.h>

struct big { int v[8]; };

struct big make(int base) {
    struct big b;
    for (int i = 0; i < 8; i++) b.v[i] = base + i;
    return b;
}

struct big twice(struct big b) {
    for (int i = 0; i < 8; i++) b.v[i] *= 2;
    return b;
}

int main(void) {
    struct big b = twice(make(10));
    for (int i = 0; i < 8; i++) printf("%d ", b.v[i]);
    printf("\n%d\n", make(3).v[7]);
    return 0;
}
