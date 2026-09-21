// atomic (the default) and nonatomic properties of object, scalar and
// struct type behave the same from one thread.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { double x, y, z; } Vec3;

@interface Body : NSObject
@property Vec3 position;
@property (nonatomic) Vec3 velocity;
@property int mass;
@property (copy) NSString *label;
@end
@implementation Body
@end

int main(void) {
    @autoreleasepool {
        Body *b = [Body new];
        b.position = (Vec3){ 1, 2, 3 };
        b.velocity = (Vec3){ -1, 0.5, 4 };
        b.mass = 80;
        b.label = @"probe";
        Vec3 p = b.position, v = b.velocity;
        printf("%g %g %g / %g %g %g / %d %s\n", p.x, p.y, p.z, v.x, v.y, v.z, b.mass,
               b.label.UTF8String);
    }
    return 0;
}
