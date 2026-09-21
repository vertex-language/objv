// dispatch_once for a singleton, and dispatch_sync on a serial queue,
// which runs its block before returning.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Shared : NSObject
+ (instancetype)shared;
@property (nonatomic) int hits;
@end
@implementation Shared
+ (instancetype)shared {
    static Shared *instance;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ instance = [Shared new]; printf("created\n"); });
    return instance;
}
@end

int main(void) {
    @autoreleasepool {
        for (int i = 0; i < 3; i++) [Shared shared].hits++;
        printf("%d %d\n", [Shared shared].hits, [Shared shared] == [Shared shared]);
        dispatch_queue_t q = dispatch_queue_create("corpus.serial", DISPATCH_QUEUE_SERIAL);
        __block int order = 0;
        for (int i = 0; i < 3; i++) dispatch_sync(q, ^{ order = order * 10 + i + 1; });
        printf("%d\n", order);
#if !__has_feature(objc_arc)
        dispatch_release(q);
#endif
    }
    return 0;
}
