// Positional and designated struct initializers; missing members are zero.
#include <stdio.h>

struct rec { int id; double w; char tag; const char *name; };

int main(void) {
    struct rec a = { 1, 2.5, 'a', "first" };
    struct rec b = { .name = "second", .id = 2 };
    struct rec c = { 0 };
    printf("%d %.1f %c %s\n", a.id, a.w, a.tag, a.name);
    printf("%d %.1f %d %s\n", b.id, b.w, b.tag, b.name);
    printf("%d %d\n", c.id, c.name == 0);
    return 0;
}
