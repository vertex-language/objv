// An 8-byte struct passed and returned by value travels in one register.
#include <stdio.h>

struct pair { int a, b; };

struct pair swap(struct pair p) { return (struct pair){ p.b, p.a }; }
int total(struct pair p, struct pair q) { return p.a + p.b + q.a + q.b; }

int main(void) {
    struct pair p = { 1, 2 };
    struct pair s = swap(p);
    printf("%d %d %d\n", s.a, s.b, total(p, s));
    return 0;
}
