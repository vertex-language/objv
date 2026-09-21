// A category adds methods to a class, including one of Foundation's.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Counter : NSObject
@property (nonatomic) int n;
@end
@implementation Counter
@end

@interface Counter (Stepping)
- (void)step;
@end
@implementation Counter (Stepping)
- (void)step { self.n += 2; }
@end

@interface NSString (Shout)
- (NSString *)shout;
@end
@implementation NSString (Shout)
- (NSString *)shout { return [[self uppercaseString] stringByAppendingString:@"!"]; }
@end

int main(void) {
    @autoreleasepool {
        Counter *c = [Counter new];
        [c step];
        [c step];
        printf("%d %s\n", c.n, [@"hey" shout].UTF8String);
    }
    return 0;
}
