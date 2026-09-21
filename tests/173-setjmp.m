// setjmp and longjmp across several frames; a volatile local keeps the value
// it had at the jump.
#include <stdio.h>
#include <setjmp.h>

static jmp_buf env;

static void deep(int n) {
    if (n == 0) longjmp(env, 42);
    deep(n - 1);
    printf("never\n");
}

int main(void) {
    volatile int tries = 0;
    int r = setjmp(env);
    tries++;
    printf("setjmp returned %d, tries %d\n", r, tries);
    if (r == 0) deep(5);
    else if (tries < 3) longjmp(env, r + 1);
    printf("done %d\n", tries);
    return 0;
}
