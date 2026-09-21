// A pointer to a whole array steps a row at a time.
#include <stdio.h>

int row_sum(int (*row)[4]) {
    int s = 0;
    for (int i = 0; i < 4; i++) s += (*row)[i];
    return s;
}

int main(void) {
    int m[3][4] = { { 1, 2, 3, 4 }, { 5, 6, 7, 8 }, { 9, 10, 11, 12 } };
    int (*p)[4] = m;
    printf("%d %d\n", row_sum(p), row_sum(p + 2));
    p++;
    printf("%d %zu\n", (*p)[3], sizeof *p);
    int *flat = &m[0][0];
    printf("%d %td\n", flat[11], (char *)(p + 1) - (char *)p);
    return 0;
}
