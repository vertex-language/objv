// Structs as message arguments: NSRange, CGPoint/CGRect-shaped HFAs, and one
// passed by reference.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { long v[4]; } Big;

@interface Geo : NSObject
- (double)area:(NSRect)r;
- (NSPoint)mid:(NSPoint)a and:(NSPoint)b;
- (long)sum:(Big)b plus:(NSRange)r;
@end
@implementation Geo
- (double)area:(NSRect)r { return r.size.width * r.size.height; }
- (NSPoint)mid:(NSPoint)a and:(NSPoint)b { return NSMakePoint((a.x + b.x) / 2, (a.y + b.y) / 2); }
- (long)sum:(Big)b plus:(NSRange)r { return b.v[0] + b.v[3] + (long)r.location + (long)r.length; }
@end

int main(void) {
    @autoreleasepool {
        Geo *g = [Geo new];
        NSPoint m = [g mid:NSMakePoint(0, 0) and:NSMakePoint(3, 5)];
        printf("%g %g %g\n", [g area:NSMakeRect(1, 1, 4, 2.5)], m.x, m.y);
        printf("%ld\n", [g sum:(Big){ { 1, 2, 3, 4 } } plus:NSMakeRange(10, 20)]);
        NSString *s = @"hello, world";
        printf("%s\n", [s substringWithRange:NSMakeRange(7, 5)].UTF8String);
    }
    return 0;
}
