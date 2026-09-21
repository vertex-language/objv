// dealloc runs when the last reference goes, and a subclass's runs before
// its superclass's.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Base : NSObject
@end
@implementation Base
- (void)dealloc {
    printf("Base dealloc\n");
#if !__has_feature(objc_arc)
    [super dealloc];
#endif
}
@end

@interface Derived : Base
@end
@implementation Derived
- (void)dealloc {
    printf("Derived dealloc\n");
#if !__has_feature(objc_arc)
    [super dealloc];
#endif
}
@end

int main(void) {
    Derived *d = [Derived new];
    printf("made\n");
#if __has_feature(objc_arc)
    d = nil;
#else
    [d release];
#endif
    printf("done\n");
    return 0;
}
