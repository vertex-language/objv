// Sending to super calls the superclass's implementation with the same self.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>

@interface Animal : NSObject
- (int)legs;
- (void)describe;
@end
@implementation Animal
- (int)legs { return 4; }
- (void)describe { printf("%s with %d legs\n", class_getName([self class]), [self legs]); }
@end

@interface Bird : Animal
@end
@implementation Bird
- (int)legs { return [super legs] - 2; }
- (void)describe { printf("bird: "); [super describe]; }
@end

int main(void) {
    [[Animal new] describe];
    [[Bird new] describe];
    return 0;
}
