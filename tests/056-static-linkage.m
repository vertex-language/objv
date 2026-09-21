// static functions and const globals.
#include <stdio.h>

static const int table[] = { 2, 3, 5, 7, 11, 13 };
static int twice(int v) { return v * 2; }

int main(void) {
    int s = 0;
    for (unsigned i = 0; i < sizeof table / sizeof table[0]; i++) s += twice(table[i]);
    printf("%d\n", s);
    return 0;
}
