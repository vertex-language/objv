// File-scope pointers initialized to an address plus a constant: an array
// element, a member, pointer arithmetic, a cast and a string literal.
#include <stdio.h>

struct point { short x, y; };
struct shape { int n; struct point pts[3]; double w; };

int arr[5] = { 10, 20, 30, 40, 50 };
struct shape sh = { 3, { { 1, 2 }, { 3, 4 }, { 5, 6 } }, 2.5 };

int *second = arr + 1;
int *end = &arr[5];
int *before_end = &arr[5] - 2;
short *y1 = &sh.pts[1].y;
double *w = &sh.w;
struct point *last = &sh.pts[2];
char *bytes = (char *)&arr + 8;
const char *tail = "address" + 3;
const char *ch = &"constant"[5];
int *const table[] = { &arr[0], &arr[4], arr + 2 };

int main(void) {
    printf("%d %td %d\n", *second, end - arr, *before_end);
    printf("%d %g %d %d\n", *y1, *w, last->x, *(int *)bytes);
    printf("%s %s\n", tail, ch);
    printf("%d %d %d\n", *table[0], *table[1], *table[2]);
    return 0;
}
