// A tagged union: a struct with a kind and a union of payloads, switched on.
#include <stdio.h>

enum kind { INT, REAL, TEXT, PAIR };
struct value {
    enum kind kind;
    union {
        long i;
        double r;
        const char *t;
        struct { int a, b; } p;
    } as;
};

static void print(struct value v) {
    switch (v.kind) {
    case INT:  printf("int %ld\n", v.as.i); break;
    case REAL: printf("real %g\n", v.as.r); break;
    case TEXT: printf("text %s\n", v.as.t); break;
    case PAIR: printf("pair %d %d\n", v.as.p.a, v.as.p.b); break;
    }
}

int main(void) {
    struct value vs[] = {
        { INT, { .i = -42 } },
        { REAL, { .r = 0.125 } },
        { TEXT, { .t = "tagged" } },
        { PAIR, { .p = { 3, 4 } } },
    };
    for (int i = 0; i < 4; i++) print(vs[i]);
    printf("%zu\n", sizeof(struct value));
    return 0;
}
