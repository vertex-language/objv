// Compound literals: an unnamed object that lives to the end of the block.
#include <stdio.h>

struct pt { int x, y; };

int sum(const int *v, int n) { int s = 0; for (int i = 0; i < n; i++) s += v[i]; return s; }

int main(void) {
    printf("%d\n", sum((int[]){ 1, 2, 3, 4 }, 4));
    struct pt *p = &(struct pt){ 5, 6 };
    p->x *= 10;
    printf("%d %d\n", p->x, p->y);
    for (int i = 0; i < 2; i++) {
        int *v = (int[3]){ i, i + 1 };
        printf("%d %d %d\n", v[0], v[1], v[2]);
    }
    return 0;
}
