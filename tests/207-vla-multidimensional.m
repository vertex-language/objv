// Multi-dimensional variable-length arrays: indexing by run-time row size,
// a pointer to a row stepping over rows, sizeof of the whole and of a row,
// and sizes that change on every trip round a loop.
#include <stdio.h>

static long fill(int rows, int cols, int depth) {
    int cube[rows][cols][depth];
    for (int r = 0; r < rows; r++)
        for (int c = 0; c < cols; c++)
            for (int d = 0; d < depth; d++) cube[r][c][d] = r * 100 + c * 10 + d;
    long s = 0;
    for (int r = 0; r < rows; r++) s += cube[r][cols - 1][depth - 1];
    printf("cube %zu %zu %zu %d\n", sizeof cube, sizeof cube[0], sizeof cube[0][0],
           cube[rows - 1][0][depth - 1]);
    return s;
}

int main(void) {
    int rows = 3, cols = 5;
    double m[rows][cols];
    for (int r = 0; r < rows; r++)
        for (int c = 0; c < cols; c++) m[r][c] = r + c / 10.0;
    printf("%g %g %zu %zu\n", m[2][4], m[1][0], sizeof m, sizeof m[1]);
    double (*row)[cols] = m;
    row++;
    printf("%g %td\n", (*row)[3], row - m);
    row += 1;
    printf("%g\n", row[0][1]);
    for (int n = 1; n <= 3; n++) printf("%ld\n", fill(n + 1, n + 2, n));
    int fixed_outer[2][cols];
    fixed_outer[1][cols - 1] = 77;
    printf("%zu %d\n", sizeof fixed_outer, fixed_outer[1][cols - 1]);
    return 0;
}
