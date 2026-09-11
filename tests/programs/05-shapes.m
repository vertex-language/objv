// Geometry: the C substrate under Objective-C, which is where a compiler's
// ABI is most easily wrong.
//
// Structs returned by value and passed by value, a struct small enough for
// registers beside one that is not, a union, an enum with an explicit
// underlying type, a bit-field, a C array walked by pointer, and qsort with
// a function pointer — all of it reached through methods, so every one of
// them crosses objc_msgSend.

#import <Foundation/Foundation.h>

typedef struct { double x, y; } Vec2;          // two doubles: registers
typedef struct { Vec2 a, b; } Span;         // four: still registers
typedef struct { double m[9]; char tag; } Slab; // too big: memory

typedef NS_ENUM(uint8_t, Axis) { AxisX = 0, AxisY = 1 };

typedef union {
	double d;
	uint64_t bits;
} DoubleBits;

typedef struct {
	unsigned visible : 1;
	unsigned locked  : 1;
	unsigned layer   : 6;
} Flags;

@interface Shape : NSObject
@property (nonatomic, assign) Vec2 origin;
@property (nonatomic, assign) Flags flags;
- (Span)diagonalTo:(Vec2)p;
- (double)spanAlong:(Axis)axis of:(Span)s;
- (Slab)slabWithTag:(char)tag;
@end

@implementation Shape

- (Span)diagonalTo:(Vec2)p {
	Span s;
	s.a = self.origin;
	s.b = p;
	return s;
}

- (double)spanAlong:(Axis)axis of:(Span)s {
	return axis == AxisX ? s.b.x - s.a.x : s.b.y - s.a.y;
}

- (Slab)slabWithTag:(char)tag {
	Slab b;
	for (int i = 0; i < 9; i++) b.m[i] = i * self.origin.x;
	b.tag = tag;
	return b;
}

@end

static int cmpDouble(const void *a, const void *b) {
	double x = *(const double *)a, y = *(const double *)b;
	return x < y ? -1 : (x > y ? 1 : 0);
}

int main(void) {
	@autoreleasepool {
		Shape *sh = [[Shape alloc] init];
		sh.origin = (Vec2){2.0, 3.0};
		sh.flags = (Flags){.visible = 1, .locked = 0, .layer = 41};

		Span d = [sh diagonalTo:(Vec2){10.0, 7.5}];
		printf("seg (%.1f,%.1f)-(%.1f,%.1f)\n", d.a.x, d.a.y, d.b.x, d.b.y);
		printf("span x=%.1f y=%.1f\n",
		       [sh spanAlong:AxisX of:d], [sh spanAlong:AxisY of:d]);

		Slab b = [sh slabWithTag:'z'];
		qsort(b.m, 9, sizeof(double), cmpDouble);
		printf("blob tag=%c first=%.1f last=%.1f\n", b.tag, b.m[0], b.m[8]);

		Flags f = sh.flags;
		printf("flags v=%u l=%u layer=%u size=%zu\n",
		       f.visible, f.locked, f.layer, sizeof(Flags));

		DoubleBits db;
		db.d = 1.0;
		printf("1.0 bits = %llx\n", (unsigned long long)db.bits);

		printf("sizes: Vec2=%zu Span=%zu Slab=%zu Axis=%zu\n",
		       sizeof(Vec2), sizeof(Span), sizeof(Slab), sizeof(Axis));
	}
	return 0;
}
