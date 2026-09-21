// Cases fall through until a break.
#include <stdio.h>

int score(int level) {
    int s = 0;
    switch (level) {
    case 3: s += 100;
    case 2: s += 10;
    case 1: s += 1; break;
    case 0: s = -1;
    }
    return s;
}

int main(void) {
    for (int i = 0; i < 5; i++) printf("%d\n", score(i));
    return 0;
}
