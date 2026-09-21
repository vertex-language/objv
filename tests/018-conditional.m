// The conditional operator evaluates only the arm it picks.
#include <stdio.h>

static int calls;
int f(int v) { calls++; return v; }

int max(int a, int b) { return a > b ? a : b; }

int main(void) {
    printf("%d %d\n", max(3, 9), max(-1, -8));
    int v = 1 ? f(10) : f(20);
    printf("%d %d\n", v, calls);
    return 0;
}
