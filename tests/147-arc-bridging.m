// ARC and Core Foundation: __bridge, __bridge_retained, __bridge_transfer
// and CFBridgingRelease move ownership exactly as they say.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@end
@implementation Obj
- (void)dealloc { printf("dealloc\n"); }
@end

int main(void) {
    @autoreleasepool {
        CFStringRef cf = CFStringCreateWithCString(NULL, "core", kCFStringEncodingUTF8);
        NSString *borrowed = (__bridge NSString *)cf;
        printf("%s %ld\n", borrowed.UTF8String, (long)CFGetRetainCount(cf));
        NSString *owned = (__bridge_transfer NSString *)cf;
        printf("%s %lu\n", owned.UTF8String, (unsigned long)owned.length);

        CFTypeRef held;
        {
            Obj *o = [Obj new];
            held = (__bridge_retained CFTypeRef)o;
        }
        printf("held past scope\n");
        // Its own pool: CFBridgingRelease is always_inline, and whether its
        // result skips the pool depends on inlining, which C leaves open.
        // Either way the object is gone when this pool is.
        @autoreleasepool {
            id back = CFBridgingRelease(held);
            printf("released to ARC %d\n", back != nil);
            back = nil;
        }
        printf("bridged pool drained\n");

        CFArrayRef arr = (__bridge CFArrayRef)@[ @1, @2, @3 ];
        printf("%ld\n", (long)CFArrayGetCount(arr));
    }
    printf("end\n");
    return 0;
}
