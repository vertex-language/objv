// Widening a signed value extends the sign; widening an unsigned one does
// not.
#include <stdio.h>
#include <stdint.h>

int64_t  widen_s(int8_t v)  { return v; }
uint64_t widen_u(uint8_t v) { return v; }
int64_t  widen_i(int32_t v) { return v; }
uint64_t widen_ui(uint32_t v) { return v; }

int main(void) {
    printf("%lld %llu\n", (long long)widen_s(-5), (unsigned long long)widen_u(251));
    printf("%lld %llu\n", (long long)widen_i(-1), (unsigned long long)widen_ui(0xFFFFFFFFu));
    printf("%llu\n", (unsigned long long)(uint64_t)(int32_t)-2);
    return 0;
}
