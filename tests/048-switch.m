// switch with a default.
#include <stdio.h>

const char *name(int d) {
    switch (d) {
    case 0: return "zero";
    case 1: return "one";
    case 2: return "two";
    default: return "many";
    }
}

int main(void) {
    for (int i = -1; i < 4; i++) printf("%s\n", name(i));
    return 0;
}
