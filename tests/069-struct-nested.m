// Structs inside structs, and arrays of structs.
#include <stdio.h>

struct point { int x, y; };
struct rect { struct point origin, size; };
struct poly { int n; struct point pts[4]; };

int main(void) {
    struct rect r = { { 1, 2 }, { 30, 40 } };
    struct poly p = { 3, { { 0, 0 }, { 5, 0 }, { 0, 5 } } };
    printf("%d %d\n", r.origin.y, r.size.x);
    printf("%d %d %d\n", p.n, p.pts[1].x, p.pts[2].y);
    struct rect *rp = &r;
    rp->size.y *= 2;
    printf("%d\n", r.size.y);
    return 0;
}
