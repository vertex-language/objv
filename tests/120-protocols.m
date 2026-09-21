// Protocols: conformance, required and optional methods, and a protocol-
// typed variable.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@protocol Speaker <NSObject>
- (const char *)sound;
@optional
- (int)volume;
@end

@interface Dog : NSObject <Speaker>
@end
@implementation Dog
- (const char *)sound { return "woof"; }
- (int)volume { return 9; }
@end

@interface Cat : NSObject <Speaker>
@end
@implementation Cat
- (const char *)sound { return "meow"; }
@end

int main(void) {
    id<Speaker> all[] = { [Dog new], [Cat new] };
    for (int i = 0; i < 2; i++) {
        id<Speaker> s = all[i];
        int v = [s respondsToSelector:@selector(volume)] ? [s volume] : 0;
        printf("%s %d %d\n", [s sound], v, [s conformsToProtocol:@protocol(Speaker)]);
    }
    printf("%d\n", [NSObject conformsToProtocol:@protocol(Speaker)]);
    return 0;
}
