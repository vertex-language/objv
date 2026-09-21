// Function pointers that take and return structs, called through a table.
#include <stdio.h>

typedef struct { double x, y; } V2;
typedef struct { long a, b, c; } L3;

static V2 add(V2 p, V2 q) { return (V2){ p.x + q.x, p.y + q.y }; }
static V2 sub(V2 p, V2 q) { return (V2){ p.x - q.x, p.y - q.y }; }
static L3 spread(long v) { return (L3){ v, v * 2, v * 3 }; }

int main(void) {
    V2 (*ops[])(V2, V2) = { add, sub };
    for (int i = 0; i < 2; i++) {
        V2 r = ops[i]((V2){ 5, 1 }, (V2){ 2, 0.5 });
        printf("%g %g\n", r.x, r.y);
    }
    L3 (*f)(long) = spread;
    L3 l = f(7);
    printf("%ld %ld %ld\n", l.a, l.b, l.c);
    return 0;
}
