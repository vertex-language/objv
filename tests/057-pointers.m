// Address-of, dereference, and a write through a pointer parameter.
#include <stdio.h>

void set(int *p, int v) { *p = v; }
void swap(int *a, int *b) { int t = *a; *a = *b; *b = t; }

int main(void) {
    int x = 1, y = 2;
    int *p = &x;
    *p += 10;
    set(&y, 20);
    swap(&x, &y);
    printf("%d %d %d\n", x, y, *p);
    return 0;
}
