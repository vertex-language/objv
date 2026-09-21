// Enums with negative and gaps in their values, switched on, and an enum
// with a fixed 64-bit underlying type.
#include <stdio.h>
#include <stdint.h>

enum temp { FREEZING = -10, COLD = 0, WARM = 20, HOT = 35 };
enum big : uint64_t { SMALL = 1, HUGE = 1ull << 50 };

const char *feel(enum temp t) {
    switch (t) {
    case FREEZING: return "freezing";
    case COLD: return "cold";
    case WARM: return "warm";
    case HOT: return "hot";
    }
    return "?";
}

int main(void) {
    enum temp all[] = { HOT, FREEZING, WARM, COLD };
    for (int i = 0; i < 4; i++) printf("%d %s\n", all[i], feel(all[i]));
    enum big b = HUGE;
    printf("%zu %llu\n", sizeof b, (unsigned long long)(b >> 40));
    return 0;
}
