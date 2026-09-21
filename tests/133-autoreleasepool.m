// Objects autoreleased inside a pool are released when the pool ends, and
// nested pools drain innermost first.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Tmp : NSObject
@property (nonatomic) int n;
@end
@implementation Tmp
- (void)dealloc {
    printf("dealloc %d\n", _n);
#if !__has_feature(objc_arc)
    [super dealloc];
#endif
}
@end

static void make(int n) {
    Tmp *t = [Tmp new];
    t.n = n;
#if __has_feature(objc_arc)
    CFAutorelease((__bridge_retained CFTypeRef)t);
#else
    [t autorelease];
#endif
}

int main(void) {
    @autoreleasepool {
        make(1);
        @autoreleasepool {
            make(2);
            printf("inner end\n");
        }
        printf("outer end\n");
    }
    printf("done\n");
    return 0;
}
