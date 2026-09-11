// An expression evaluator: recursion, a C struct stack, integer overflow
// behaviour, and the arithmetic conversions.
//
// Almost no Objective-C in it on purpose. Half of any real program is C, and
// the half that is C is where the promotions, the shifts and the division
// rules live — every one of which has a rule the language states and a
// compiler can get wrong without any test noticing.

#import <Foundation/Foundation.h>

enum { StackMax = 32 };

typedef struct {
	long items[StackMax];
	int top;
} Stack;

static void push(Stack *s, long v) { if (s->top < StackMax) s->items[s->top++] = v; }
static long pop(Stack *s)          { return s->top > 0 ? s->items[--s->top] : 0; }

// Reverse Polish, which is a parser small enough to read and a switch big
// enough to matter.
static long evalRPN(const char *expr, BOOL *ok) {
	Stack s = {{0}, 0};
	*ok = YES;
	for (const char *p = expr; *p; p++) {
		if (*p == ' ') continue;
		if (*p >= '0' && *p <= '9') { push(&s, *p - '0'); continue; }
		long b = pop(&s), a = pop(&s);
		switch (*p) {
		case '+': push(&s, a + b); break;
		case '-': push(&s, a - b); break;
		case '*': push(&s, a * b); break;
		case '/':
			if (b == 0) { *ok = NO; return 0; }
			push(&s, a / b);
			break;
		case '%':
			if (b == 0) { *ok = NO; return 0; }
			push(&s, a % b);
			break;
		default:
			*ok = NO;
			return 0;
		}
	}
	return pop(&s);
}

static unsigned long fib(unsigned n) { return n < 2 ? n : fib(n - 1) + fib(n - 2); }

static unsigned popcount64(unsigned long long v) {
	unsigned n = 0;
	while (v) { n += (unsigned)(v & 1); v >>= 1; }
	return n;
}

int main(void) {
	@autoreleasepool {
		const char *exprs[] = {"34+", "92-5*", "84/2%", "80/", "8?", NULL};
		for (int i = 0; exprs[i]; i++) {
			BOOL ok;
			long v = evalRPN(exprs[i], &ok);
			printf("%-6s = %ld (%s)\n", exprs[i], v, ok ? "ok" : "error");
		}

		printf("fib: ");
		for (unsigned i = 0; i < 12; i++) printf("%lu ", fib(i));
		printf("\n");

		// Conversions and the rules around them.
		int i = -1;
		unsigned u = 1;
		printf("cmp=%d  (int)-1 as unsigned = %u\n", i < (int)u, (unsigned)i);

		signed char sc = -3;
		short sh = (short)(sc * 1000);
		printf("sc=%d sh=%d promoted=%d\n", sc, sh, sc * 1000);

		printf("div: %d %d %d %d\n", 7 / 2, -7 / 2, 7 % 2, -7 % 2);
		printf("shift: %d %ld %u\n", 1 << 10, -8L >> 2, 0x80000000u >> 31);

		double d = 2.5;
		printf("casts: %d %ld %.1f %.1f\n",
		       (int)d, (long)(d * 1e9), (double)(int)d, (float)0.1);

		unsigned long long bits = 0xF0F0F0F0F0F0F0F0ull;
		printf("popcount=%u hex=%llx\n", popcount64(bits), bits);

		// Wrapping, which is defined for unsigned and is what a hash does.
		unsigned h = 2166136261u;
		for (const char *p = "objv"; *p; p++) { h ^= (unsigned char)*p; h *= 16777619u; }
		printf("fnv=%u\n", h);

		printf("limits: %d %ld %u %llu\n", INT_MIN, LONG_MAX, UINT_MAX, ULLONG_MAX);
	}
	return 0;
}
