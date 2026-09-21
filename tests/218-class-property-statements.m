// A statement that begins with a class's name and a dot is an expression --
// a class property -- in every form: ++, compound assignment, a message to
// the property's value, and an object-valued one tested in a condition.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

static int gCount;
static NSString *gName;

@interface Registry : NSObject
@property (class, nonatomic) int count;
@property (class, nonatomic, copy) NSString *name;
@end
@implementation Registry
+ (int)count { return gCount; }
+ (void)setCount:(int)c { gCount = c; }
+ (NSString *)name { return gName; }
+ (void)setName:(NSString *)n {
#if __has_feature(objc_arc)
    gName = [n copy];
#else
    [gName release];
    gName = [n copy];
#endif
}
@end

int main(void) {
    @autoreleasepool {
        Registry.count++;
        Registry.count += 5;
        Registry.count *= 2;
        Registry.name = @"registry";
        Registry.name.length;
        if (Registry.name) printf("%s %lu\n", Registry.name.UTF8String, (unsigned long)Registry.name.length);
        printf("%d\n", Registry.count);
    }
    return 0;
}
