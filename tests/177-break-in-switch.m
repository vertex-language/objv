// break inside a switch leaves the switch, not the loop around it; continue
// inside a switch continues the loop.
#include <stdio.h>

int main(void) {
    for (int i = 0; i < 6; i++) {
        switch (i % 3) {
        case 0:
            printf("zero ");
            break;
        case 1:
            if (i > 3) continue;
            printf("one ");
            break;
        default:
            printf("other ");
        }
        printf("[%d]\n", i);
    }
    int i = 0;
    while (1) {
        switch (i++) {
        case 4: goto out;
        default: break;
        }
    }
out:
    printf("out at %d\n", i);
    return 0;
}
