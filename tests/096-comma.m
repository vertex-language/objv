// The comma operator evaluates left to right and yields the right.
#include <stdio.h>

static int trace[4], n;
int mark(int v) { trace[n++] = v; return v; }

int main(void) {
    int r = (mark(1), mark(2), mark(3));
    printf("%d %d %d %d\n", r, trace[0], trace[1], trace[2]);
    int i, j;
    for (i = 0, j = 10; i < j; i += 3, j -= 3) {}
    printf("%d %d\n", i, j);
    return 0;
}
