// float and double results from messages, and float arguments to them.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Calc : NSObject
- (float)half:(float)v;
- (double)mix:(double)a with:(float)b weight:(int)w;
+ (double)pi;
@end
@implementation Calc
- (float)half:(float)v { return v / 2; }
- (double)mix:(double)a with:(float)b weight:(int)w { return a * w + b; }
+ (double)pi { return 3.141592653589793; }
@end

int main(void) {
    Calc *c = [Calc new];
    printf("%.9g %.17g %.17g\n", [c half:3.0f], [c mix:1.25 with:0.5f weight:4], [Calc pi]);
    double sum = [c half:1] + [c half:2] * [Calc pi];
    printf("%.17g\n", sum);
    return 0;
}
