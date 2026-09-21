// const and volatile qualifiers: volatile reads happen every time.
#include <stdio.h>

static volatile int ticks;
void tick(void) { ticks++; }

int main(void) {
    const int limit = 5;
    const char *const name = "const";
    while (ticks < limit) tick();
    volatile int v = 1;
    int s = v + v + v;
    printf("%d %s %d\n", ticks, name, s);
    const int *p = &limit;
    printf("%d\n", *p);
    return 0;
}
