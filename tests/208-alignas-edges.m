// Alignment asked for by name: _Alignas(n) and _Alignas(type), the aligned
// attribute, on members (changing layout and array stride), on locals past
// the stack's own 16 bytes, on static locals and on globals.
#include <stdio.h>
#include <stdint.h>
#include <stddef.h>

struct m { char c; _Alignas(double) char d; __attribute__((aligned(32))) short s; };
struct big { int x; _Alignas(64) char tail; };

__attribute__((aligned(128))) int g_attr = 1;
_Alignas(32) static double g_arr[3];

static int aligned_mod(const void *p, int n) { return (int)((uintptr_t)p % n); }

static struct big make_big(int x) { struct big b = { x, 'q' }; return b; }
static int take_big(struct big b) { return b.x + aligned_mod(&b.tail, 64); }

static int depth(int n) {
    _Alignas(64) char local[3];
    char other;
    local[0] = (char)n;
    int r = aligned_mod(local, 64);
    (void)other;
    return n == 0 ? r : r + depth(n - 1);
}

int main(void) {
    printf("%zu %zu %zu %zu\n", sizeof(struct m), _Alignof(struct m),
           offsetof(struct m, d), offsetof(struct m, s));
    printf("%zu %zu\n", sizeof(struct big), offsetof(struct big, tail));
    struct big arr[2];
    printf("%td %d\n", (char *)&arr[1] - (char *)&arr[0], aligned_mod(&arr[1].tail, 64));
    printf("%d %d\n", aligned_mod(&g_attr, 128), aligned_mod(g_arr, 32));
    printf("%d\n", depth(5));
    printf("%d %c\n", take_big(make_big(7)), make_big(1).tail);
    static _Alignas(256) char s_local[4];
    __attribute__((aligned(16))) char a16[3];
    printf("%d %d\n", aligned_mod(s_local, 256), aligned_mod(a16, 16));
    return 0;
}
