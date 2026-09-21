// A global with nested structs, arrays and designators, some members left
// to zero.
#include <stdio.h>

struct point { short x, y; };
struct shape {
    char name[8];
    int n;
    struct point pts[4];
    double scale;
};

struct shape shapes[3] = {
    { "tri", 3, { { 0, 0 }, { 4, 0 }, { 0, 3 } }, 1.0 },
    [2] = { .name = "dot", .n = 1, .pts[0] = { .y = -5 } },
};

int main(void) {
    for (int i = 0; i < 3; i++) {
        struct shape *s = &shapes[i];
        printf("[%s] %d %g:", s->name, s->n, s->scale);
        for (int j = 0; j < 4; j++) printf(" (%d,%d)", s->pts[j].x, s->pts[j].y);
        printf("\n");
    }
    return 0;
}
