// A conditional whose value is a struct.
//
// `marked ? NSMakeRange(0, n) : NSMakeRange(NSNotFound, 0)` is how
// NSTextInputClient's markedRange is written in every text view there is,
// and the same shape picks a rect, a point or a colour triple. A struct is
// not a register, so the two arms cannot hand one to the join the way a
// scalar conditional does; the result is storage of its own that each arm
// fills. It was rejected outright.

#import <Foundation/Foundation.h>

typedef struct {
    double r, g, b;
} Colour;

static Colour colour(double r, double g, double b) {
    Colour c = { r, g, b };
    return c;
}

@interface Composer : NSObject
@property (nonatomic, copy) NSString *marked;
- (NSRange)markedRange;
@end

@implementation Composer
- (NSRange)markedRange {
    return self.marked != nil ? NSMakeRange(0, [self.marked length]) : NSMakeRange(NSNotFound, 0);
}
@end

int main(void) {
    @autoreleasepool {
        Composer *c = [[Composer alloc] init];
        NSRange none = [c markedRange];
        printf("none: %d %lu\n", none.location == NSNotFound, (unsigned long)none.length);
        c.marked = @"かな";
        NSRange some = [c markedRange];
        printf("some: %lu %lu\n", (unsigned long)some.location, (unsigned long)some.length);

        // Nested, and used directly as an argument without a variable.
        for (int dark = 0; dark < 2; dark++) {
            for (int high = 0; high < 2; high++) {
                Colour k = dark ? (high ? colour(1, 1, 1) : colour(0.8, 0.8, 0.8))
                                : (high ? colour(0, 0, 0) : colour(0.2, 0.2, 0.2));
                printf("%d%d: %.1f %.1f %.1f\n", dark, high, k.r, k.g, k.b);
            }
        }
        NSRect r = (some.length > 1) ? NSMakeRect(1, 2, 3, 4) : NSZeroRect;
        printf("rect: %.0f %.0f %.0f %.0f\n", r.origin.x, r.origin.y, r.size.width, r.size.height);

        // The result is a copy, not an alias of the arm that ran.
        Colour a = colour(0.5, 0.5, 0.5);
        Colour b = colour(0.1, 0.1, 0.1);
        Colour pick = 1 ? a : b;
        a.r = 9;
        printf("copy: %.1f %.1f\n", pick.r, a.r);
    }
    return 0;
}
