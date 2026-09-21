// A class method, sent to the class object, and +new.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>

@interface Math : NSObject
+ (int)square:(int)v;
+ (Class)me;
@end

@implementation Math
+ (int)square:(int)v { return v * v; }
+ (Class)me { return self; }
@end

int main(void) {
    printf("%d\n", [Math square:9]);
    printf("%d\n", [Math me] == [Math class]);
    Math *m = [Math new];
    printf("%s\n", class_getName([m class]));
    return 0;
}
