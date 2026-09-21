// isEqual: and hash overridden together make distinct objects one key in a
// set and a dictionary.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Key : NSObject <NSCopying>
@property (nonatomic) int x, y;
+ (instancetype)x:(int)x y:(int)y;
@end
@implementation Key
+ (instancetype)x:(int)x y:(int)y { Key *k = [self new]; k.x = x; k.y = y;
#if !__has_feature(objc_arc)
    [k autorelease];
#endif
    return k; }
- (BOOL)isEqual:(id)o { return [o isKindOfClass:[Key class]] && [o x] == _x && [o y] == _y; }
- (NSUInteger)hash { return (NSUInteger)(_x * 31 + _y); }
- (id)copyWithZone:(NSZone *)z {
#if __has_feature(objc_arc)
    return self;
#else
    return [self retain];
#endif
}
@end

int main(void) {
    @autoreleasepool {
        NSSet *s = [NSSet setWithObjects:[Key x:1 y:2], [Key x:1 y:2], [Key x:2 y:1], nil];
        printf("%lu\n", (unsigned long)s.count);
        NSMutableDictionary *d = [NSMutableDictionary dictionary];
        d[[Key x:5 y:5]] = @"first";
        d[[Key x:5 y:5]] = @"second";
        printf("%lu %s\n", (unsigned long)d.count, [d[[Key x:5 y:5]] UTF8String]);
        printf("%d %d\n", [[Key x:1 y:1] isEqual:[Key x:1 y:1]], [Key x:1 y:1] == [Key x:1 y:1]);
    }
    return 0;
}
