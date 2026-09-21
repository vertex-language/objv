// A call into libc, with a string and an int.
#include <stdio.h>

int main(void) {
    printf("%s %d\n", "hello", 42);
    return 0;
}
