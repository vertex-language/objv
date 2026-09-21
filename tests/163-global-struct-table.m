// A const global table of structs holding strings, numbers and function
// pointers, all resolved at load time.
#include <stdio.h>

static int twice(int v) { return v * 2; }
static int negate(int v) { return -v; }

struct op { const char *name; int (*fn)(int); double weight; char code; };

static const struct op ops[] = {
    { "twice", twice, 1.5, 't' },
    { "negate", negate, -0.25, 'n' },
    { "none", 0, 0, 0 },
};

static const char *const names[] = { "zero", "one", "two" };

int main(void) {
    for (int i = 0; ops[i].fn; i++)
        printf("%s %d %g %c\n", ops[i].name, ops[i].fn(21), ops[i].weight, ops[i].code);
    printf("%s %s\n", names[2], ops[2].name);
    return 0;
}
