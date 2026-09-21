// A struct with members read and written through . and ->.
#include <stdio.h>

struct point { int x, y; };

void move(struct point *p, int dx) { p->x += dx; }

int main(void) {
    struct point p;
    p.x = 3;
    p.y = 4;
    move(&p, 10);
    printf("%d %d %zu\n", p.x, p.y, sizeof p);
    return 0;
}
