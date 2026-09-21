// A static local array keeps its contents between calls.
#include <stdio.h>

int *history(int v) {
    static int seen[5] = { -1, -1, -1, -1, -1 };
    static int next;
    seen[next++ % 5] = v;
    return seen;
}

int main(void) {
    int *h = 0;
    for (int i = 1; i <= 7; i++) h = history(i * 11);
    for (int i = 0; i < 5; i++) printf("%d ", h[i]);
    printf("\n");
    return 0;
}
