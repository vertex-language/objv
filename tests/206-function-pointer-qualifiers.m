// A parameter's own qualifiers (const, volatile, restrict at the top level)
// are not part of a function's type: a pointer declared without them
// points at a function declared with them, and the other way round.
#include <stdio.h>
#include <string.h>

static int count(const int n, const char *restrict s) { return n + (int)strlen(s); }
static void fill(int *const p, volatile int v) { *p = v; }

int main(void) {
    int (*pf)(const char *, ...) = printf;
    pf("%s %d\n", "printf", 1);
    char *(*cp)(char *, const char *) = strcpy;
    char buf[16];
    cp(buf, "copied");
    printf("%s\n", buf);
    int (*c)(int, const char *) = count;
    void (*f)(int *, int) = fill;
    int x = 0;
    f(&x, 9);
    printf("%d %d\n", c(3, "abcd"), x);
    return 0;
}
