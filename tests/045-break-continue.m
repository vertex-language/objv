// break leaves the innermost loop; continue skips to its next iteration.
#include <stdio.h>

int main(void) {
    for (int i = 0; i < 10; i++) {
        if (i % 2) continue;
        if (i > 6) break;
        printf("%d ", i);
    }
    printf("\n");
    for (int i = 0; i < 3; i++)
        for (int j = 0; j < 3; j++) {
            if (j == 1) break;
            printf("(%d,%d)", i, j);
        }
    printf("\n");
    return 0;
}
