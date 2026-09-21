// +load runs before main; +initialize runs once, before a class's first
// message, and a subclass that does not define it triggers the super's.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

static int loaded, initialized;

@interface Base : NSObject
+ (void)touch;
@end
@implementation Base
+ (void)load { loaded++; }
+ (void)initialize { initialized++; printf("initialize %s\n", self == [Base class] ? "Base" : "sub"); }
+ (void)touch {}
@end

@interface Sub : Base
@end
@implementation Sub
@end

int main(void) {
    printf("main: loaded %d, initialized %d\n", loaded, initialized);
    [Base touch];
    [Base touch];
    printf("after Base %d\n", initialized);
    [Sub touch];
    printf("after Sub %d\n", initialized);
    return 0;
}
