// The smallest class: an NSObject subclass with one instance method.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Greeter : NSObject
- (int)answer;
@end

@implementation Greeter
- (int)answer { return 42; }
@end

int main(void) {
    Greeter *g = [[Greeter alloc] init];
    printf("%d\n", [g answer]);
    return 0;
}
