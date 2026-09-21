// An array parameter is a pointer; a row parameter carries its width.
#include <stdio.h>

int sum(const int a[], int n) {
    int s = 0;
    for (int i = 0; i < n; i++) s += a[i];
    return s;
}

int diag(int m[][3], int n) {
    int s = 0;
    for (int i = 0; i < n; i++) s += m[i][i];
    return s;
}

void fill(int *a, int n, int v) { for (int i = 0; i < n; i++) a[i] = v + i; }

int main(void) {
    int a[5];
    fill(a, 5, 100);
    int m[3][3] = { { 1, 2, 3 }, { 4, 5, 6 }, { 7, 8, 9 } };
    printf("%d %d\n", sum(a, 5), diag(m, 3));
    return 0;
}
