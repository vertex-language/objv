// A pointer to a pointer, used to change where the pointer points.
#include <stdio.h>

static int first = 1, second = 2;
void repoint(int **pp) { *pp = &second; }

int main(void) {
    int *p = &first;
    int **pp = &p;
    printf("%d\n", **pp);
    repoint(pp);
    printf("%d\n", *p);
    **pp = 22;
    printf("%d\n", second);
    return 0;
}
