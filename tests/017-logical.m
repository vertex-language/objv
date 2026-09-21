// && and || yield 0 or 1 and do not evaluate their right side when the left
// already decides.
#include <stdio.h>

static int calls;
int touch(int v) { calls++; return v; }

int main(void) {
    printf("%d %d %d\n", 5 && 7, 0 || 9, !3);
    int r = touch(0) && touch(1);
    r += touch(1) || touch(1);
    printf("%d %d\n", r, calls);
    return 0;
}
