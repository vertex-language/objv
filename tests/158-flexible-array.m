// A flexible array member: sizeof leaves it out, malloc makes room for it.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

struct buf { int len; char tag; double data[]; };
struct str { unsigned short n; char s[]; };

int main(void) {
    printf("%zu %zu\n", sizeof(struct buf), sizeof(struct str));
    struct buf *b = malloc(sizeof *b + 5 * sizeof(double));
    b->len = 5;
    for (int i = 0; i < 5; i++) b->data[i] = i * 1.5;
    double t = 0;
    for (int i = 0; i < b->len; i++) t += b->data[i];
    printf("%g\n", t);
    struct str *s = malloc(sizeof *s + 6);
    s->n = 5;
    memcpy(s->s, "flexy", 6);
    printf("%s %u\n", s->s, s->n);
    free(b);
    free(s);
    return 0;
}
