// continue in a do-while jumps to the condition, not the top of the body.
#include <stdio.h>

int main(void) {
    int i = 0, body = 0;
    do {
        i++;
        if (i % 2) continue;
        body++;
        printf("even %d\n", i);
    } while (i < 7);
    printf("%d %d\n", i, body);
    int j = 10;
    do { if (j == 10) { j = 99; continue; } j++; } while (j < 5);
    printf("%d\n", j);
    return 0;
}
