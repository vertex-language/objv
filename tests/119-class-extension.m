// A class extension declares private properties and methods, and redeclares
// a readonly property readwrite.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Stack : NSObject
@property (nonatomic, readonly) int depth;
- (void)push:(int)v;
- (int)pop;
@end

@interface Stack ()
@property (nonatomic, readwrite) int depth;
@property (nonatomic) int *slots;
- (void)grow;
@end

@implementation Stack {
    int _capacity;
}
- (void)grow {
    _capacity = _capacity ? _capacity * 2 : 2;
    self.slots = realloc(self.slots, _capacity * sizeof(int));
}
- (void)push:(int)v {
    if (self.depth == _capacity) [self grow];
    self.slots[self.depth++] = v;
}
- (int)pop { return self.slots[--self.depth]; }
@end

int main(void) {
    Stack *s = [Stack new];
    for (int i = 1; i <= 5; i++) [s push:i * 10];
    int a = [s pop];
    int b = [s pop];
    printf("%d %d %d\n", a, b, s.depth);
    return 0;
}
