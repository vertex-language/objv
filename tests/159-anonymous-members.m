// Anonymous structs and unions inside a struct: their members are reached
// directly.
#include <stdio.h>
#include <stddef.h>

struct vec {
    union {
        struct { float x, y, z; };
        float v[3];
    };
    int kind;
};

int main(void) {
    struct vec a = { .x = 1, .y = 2, .z = 3, .kind = 7 };
    a.v[1] = 20;
    printf("%g %g %g %d\n", a.x, a.y, a.z, a.kind);
    printf("%zu %zu %zu\n", sizeof a, offsetof(struct vec, z), offsetof(struct vec, kind));
    return 0;
}
