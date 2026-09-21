// Returning small structs of several shapes.
#include <stdio.h>

struct c3 { char a, b, c; };
struct s12 { int a, b, c; };

struct c3 letters(void) { return (struct c3){ 'x', 'y', 'z' }; }
struct s12 triple(int v) { return (struct s12){ v, v * 2, v * 3 }; }

int main(void) {
    struct c3 l = letters();
    struct s12 t = triple(7);
    printf("%c%c%c %d %d %d\n", l.a, l.b, l.c, t.a, t.b, t.c);
    return 0;
}
