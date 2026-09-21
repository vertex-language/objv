// A two-dimensional array is rows laid end to end.
#include <stdio.h>

int main(void) {
    int m[3][4];
    for (int r = 0; r < 3; r++)
        for (int c = 0; c < 4; c++) m[r][c] = r * 10 + c;
    printf("%d %d %d\n", m[2][3], m[1][0], ((int *)m)[5]);
    int id[3][3] = { { 1 }, { 0, 1 }, { 0, 0, 1 } };
    int tr = 0;
    for (int i = 0; i < 3; i++) tr += id[i][i];
    printf("%d %zu %zu\n", tr, sizeof m, sizeof m[0]);
    return 0;
}
