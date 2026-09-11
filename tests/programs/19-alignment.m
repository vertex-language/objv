// Layout questions the language answers and a compiler has to agree with:
// alignment, padding, offsets, and the bytes a memcpy actually moves.
//
// Nothing here is Objective-C except the class that carries the structs
// across a message send, which is the point — an ABI is only visible when a
// value crosses a boundary, and objc_msgSend is the boundary every
// Objective-C program has.

#import <Foundation/Foundation.h>
#import <stddef.h>

typedef struct {
	char a;
	// three bytes of padding
	int b;
	char c;
	// seven bytes, because of the double
	double d;
} Padded;

typedef struct __attribute__((packed)) {
	char a;
	int b;
	char c;
	double d;
} Packed;

typedef struct __attribute__((aligned(32))) {
	int x;
} OverAligned;

typedef struct {
	Padded head;
	char tail[3];
} Nested;

_Static_assert(sizeof(Packed) == 14, "packed has no padding");
_Static_assert(_Alignof(OverAligned) == 32, "the attribute is the alignment");

@interface Mover : NSObject
- (Padded)bump:(Padded)p by:(int)n;
- (Nested)wrap:(Padded)p tag:(char)t;
- (void)fill:(Padded *)out from:(const Padded *)in;
@end

@implementation Mover

- (Padded)bump:(Padded)p by:(int)n {
	p.a = (char)(p.a + n);
	p.b += n;
	p.c = (char)(p.c + n);
	p.d += n;
	return p;
}

- (Nested)wrap:(Padded)p tag:(char)t {
	Nested n = {.head = p};
	n.tail[0] = t;
	n.tail[1] = (char)(t + 1);
	n.tail[2] = (char)(t + 2);
	return n;
}

- (void)fill:(Padded *)out from:(const Padded *)in {
	memcpy(out, in, sizeof(Padded));
	out->b *= 2;
}

@end

static void hexdump(const char *label, const void *p, size_t n) {
	const unsigned char *b = p;
	printf("%-8s", label);
	for (size_t i = 0; i < n; i++) printf(" %02x", b[i]);
	printf("\n");
}

int main(void) {
	@autoreleasepool {
		printf("Padded  size=%zu align=%zu a=%zu b=%zu c=%zu d=%zu\n",
		       sizeof(Padded), _Alignof(Padded),
		       offsetof(Padded, a), offsetof(Padded, b),
		       offsetof(Padded, c), offsetof(Padded, d));
		printf("Packed  size=%zu align=%zu b=%zu d=%zu\n",
		       sizeof(Packed), _Alignof(Packed),
		       offsetof(Packed, b), offsetof(Packed, d));
		printf("Over    size=%zu align=%zu\n",
		       sizeof(OverAligned), _Alignof(OverAligned));
		printf("Nested  size=%zu tail=%zu\n",
		       sizeof(Nested), offsetof(Nested, tail));

		Mover *m = [[Mover alloc] init];

		// A struct across a send, both ways, with the padding intact.
		Padded p = {.a = 1, .b = 2, .c = 3, .d = 4.5};
		Padded q = [m bump:p by:10];
		printf("bumped a=%d b=%d c=%d d=%.1f (original b=%d)\n",
		       q.a, q.b, q.c, q.d, p.b);

		// One that does not fit in registers, so it comes back through the
		// caller's storage.
		Nested n = [m wrap:q tag:'x'];
		printf("nested b=%d tail=%c%c%c\n",
		       n.head.b, n.tail[0], n.tail[1], n.tail[2]);

		// The bytes themselves. Zeroed first, so the padding is a known
		// value and the dump is a fact about the layout.
		Padded z;
		memset(&z, 0, sizeof z);
		z.a = 0x11; z.b = 0x22334455; z.c = 0x66; z.d = 1.0;
		hexdump("padded", &z, sizeof z);

		Packed k;
		memset(&k, 0, sizeof k);
		k.a = 0x11; k.b = 0x22334455; k.c = 0x66; k.d = 1.0;
		hexdump("packed", &k, sizeof k);

		// Out-parameters of struct type, which is a pointer either way.
		Padded out;
		[m fill:&out from:&z];
		printf("filled b=%d d=%.1f\n", out.b, out.d);

		// An array of them, walked by pointer, so the stride is the size.
		Padded arr[3];
		for (int i = 0; i < 3; i++) {
			arr[i] = (Padded){.a = (char)i, .b = i * 100, .d = i / 2.0};
		}
		const Padded *cur = arr;
		printf("stride=%td b=%d %d %d\n",
		       (const char *)(cur + 1) - (const char *)cur,
		       arr[0].b, (cur + 1)->b, (cur + 2)->b);

		// A struct in a collection, which has to be boxed.
		NSValue *boxed = [NSValue valueWithBytes:&z objCType:@encode(Padded)];
		Padded unboxed;
		[boxed getValue:&unboxed];
		printf("boxed b=%d d=%.1f encode=%s\n", unboxed.b, unboxed.d, [boxed objCType]);
	}
	return 0;
}
