// A member of a function's result is read with the function called once:
// f().x, f().arr[i], a bit-field of f(), and p()->x through a function
// returning a pointer. Each call has a side effect that counts.
#include <stdio.h>

struct rec { int x; int arr[3]; unsigned bits : 5; };

static int calls;
static struct rec store = { 7, { 1, 2, 3 }, 9 };

static struct rec f(void) { calls++; return store; }
static struct rec *p(void) { calls++; return &store; }

int main(void) {
    int a = f().x;
    int b = f().arr[2];
    unsigned c = f().bits;
    int d = p()->x;
    p()->arr[0] += 10;
    p()->bits = 17;
    printf("%d %d %u %d %d %u calls %d\n", a, b, c, d, store.arr[0], store.bits, calls);
    return 0;
}
