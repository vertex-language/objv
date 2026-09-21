// _Bool: any nonzero value converts to 1.
#include <stdio.h>
#include <stdbool.h>

bool truthy(long v) { return v; }

int main(void) {
    printf("%d %d %d\n", truthy(0), truthy(256), truthy(-1));
    bool b = 0.5;
    printf("%d\n", b);
    bool c = true;
    c += 1;
    printf("%d %zu\n", c, sizeof(bool));
    return 0;
}
