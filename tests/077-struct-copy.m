// Struct assignment copies every member, and the copies are independent.
#include <stdio.h>

struct rec { int id; char name[12]; double score; };

int main(void) {
    struct rec a = { 1, "alpha", 9.5 };
    struct rec b = a;
    b.id = 2;
    b.name[0] = 'A';
    b.score += 1;
    printf("%d %s %.1f\n", a.id, a.name, a.score);
    printf("%d %s %.1f\n", b.id, b.name, b.score);
    struct rec arr[2];
    arr[1] = b;
    printf("%s\n", arr[1].name);
    return 0;
}
