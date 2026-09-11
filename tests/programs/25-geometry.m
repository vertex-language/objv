// CoreGraphics, which is the SDK's most struct-dense header and the one
// whose geometry a compiler cannot fake.
//
// CGGeometry.h is thirty CG_INLINE functions over four structs — every one
// of them takes and returns a struct by value, so reading the header at all
// means agreeing with clang about how a CGRect travels. CGAffineTransform is
// six CGFloats, which is past every register file this platform has.
//
// frameworks: CoreGraphics

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>

@interface Layout : NSObject
@property (nonatomic, assign) CGRect bounds;
- (CGRect)insetBy:(CGFloat)dx and:(CGFloat)dy;
- (CGPoint)anchorAt:(CGFloat)ux :(CGFloat)uy;
- (CGAffineTransform)transformTo:(CGRect)target;
@end

@implementation Layout

- (CGRect)insetBy:(CGFloat)dx and:(CGFloat)dy {
	return CGRectInset(self.bounds, dx, dy);
}

- (CGPoint)anchorAt:(CGFloat)ux :(CGFloat)uy {
	return CGPointMake(CGRectGetMinX(_bounds) + ux * CGRectGetWidth(_bounds),
	                   CGRectGetMinY(_bounds) + uy * CGRectGetHeight(_bounds));
}

// Six CGFloats out, two rects in: nothing about this fits in registers.
- (CGAffineTransform)transformTo:(CGRect)target {
	CGFloat sx = CGRectGetWidth(target) / CGRectGetWidth(_bounds);
	CGFloat sy = CGRectGetHeight(target) / CGRectGetHeight(_bounds);
	CGAffineTransform scale = CGAffineTransformMakeScale(sx, sy);
	CGAffineTransform move = CGAffineTransformMakeTranslation(
		CGRectGetMinX(target) - CGRectGetMinX(_bounds) * sx,
		CGRectGetMinY(target) - CGRectGetMinY(_bounds) * sy);
	return CGAffineTransformConcat(scale, move);
}

@end

static void showRect(const char *label, CGRect r) {
	printf("%-10s (%.2f,%.2f %.2fx%.2f)\n", label,
	       (double)r.origin.x, (double)r.origin.y,
	       (double)r.size.width, (double)r.size.height);
}

static void showTransform(const char *label, CGAffineTransform t) {
	printf("%-10s [%.2f %.2f %.2f %.2f %.2f %.2f]\n", label,
	       (double)t.a, (double)t.b, (double)t.c,
	       (double)t.d, (double)t.tx, (double)t.ty);
}

int main(void) {
	@autoreleasepool {
		printf("sizes: CGFloat=%zu CGPoint=%zu CGSize=%zu CGRect=%zu CGAffine=%zu\n",
		       sizeof(CGFloat), sizeof(CGPoint), sizeof(CGSize),
		       sizeof(CGRect), sizeof(CGAffineTransform));

		Layout *l = [[Layout alloc] init];
		l.bounds = CGRectMake(10, 20, 100, 50);
		showRect("bounds", l.bounds);
		showRect("inset", [l insetBy:5 and:2.5]);

		CGPoint c = [l anchorAt:0.5 :0.5];
		CGPoint tr = [l anchorAt:1 :1];
		printf("centre (%.2f,%.2f) corner (%.2f,%.2f)\n",
		       (double)c.x, (double)c.y, (double)tr.x, (double)tr.y);

		// The predicates, which are the inline functions the header defines
		// rather than anything a library exports.
		CGRect a = CGRectMake(0, 0, 40, 40);
		CGRect b = CGRectMake(20, 20, 40, 40);
		showRect("union", CGRectUnion(a, b));
		showRect("inter", CGRectIntersection(a, b));
		showRect("offset", CGRectOffset(a, 5, -5));
		showRect("integral", CGRectIntegral(CGRectMake(1.2, 3.7, 5.1, 2.9)));
		printf("contains=%d intersects=%d empty=%d equal=%d null=%d\n",
		       CGRectContainsPoint(a, CGPointMake(10, 10)),
		       CGRectIntersectsRect(a, b),
		       CGRectIsEmpty(CGRectMake(0, 0, 0, 10)),
		       CGRectEqualToRect(a, CGRectMake(0, 0, 40, 40)),
		       CGRectIsNull(CGRectIntersection(a, CGRectMake(500, 500, 1, 1))));

		// CGRectDivide takes two out-parameters of struct type.
		CGRect head, tail;
		CGRectDivide(CGRectMake(0, 0, 100, 20), &head, &tail, 30, CGRectMinXEdge);
		showRect("head", head);
		showRect("tail", tail);

		showTransform("identity", CGAffineTransformIdentity);
		CGAffineTransform t = [l transformTo:CGRectMake(0, 0, 200, 200)];
		showTransform("fit", t);
		showRect("applied", CGRectApplyAffineTransform(l.bounds, t));
		printf("invertible=%d identity=%d\n",
		       !CGAffineTransformEqualToTransform(CGAffineTransformInvert(t), t),
		       CGAffineTransformIsIdentity(CGAffineTransformIdentity));

		// Boxed, which is how a rect goes in a collection.
		NSValue *boxed = [NSValue valueWithBytes:&a objCType:@encode(CGRect)];
		CGRect back;
		[boxed getValue:&back];
		showRect("unboxed", back);
		printf("encode=%s\n", @encode(CGRect));
	}
	return 0;
}
