// A subclass's ivars come after its superclass's; each class reads its own
// through self, and the runtime reports the layout clang would.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>

@interface A : NSObject {
@public
    char _a;
    double _d;
}
@end
@implementation A
@end

@interface B : A {
@public
    char _b;
    int _i;
}
- (void)fill;
@end
@implementation B {
    short _hidden;
}
- (void)fill { _a = 'a'; _d = 1.5; _b = 'b'; _i = 7; _hidden = -2; }
- (int)hidden { return _hidden; }
@end

int main(void) {
    B *b = [B new];
    [b fill];
    printf("%c %g %c %d %d\n", b->_a, b->_d, b->_b, b->_i, [b hidden]);
    const char *names[] = { "_a", "_d", "_b", "_i", "_hidden" };
    for (int i = 0; i < 5; i++) {
        Ivar v = class_getInstanceVariable([B class], names[i]);
        printf("%s %td %s\n", names[i], ivar_getOffset(v), ivar_getTypeEncoding(v));
    }
    printf("%zu %zu\n", class_getInstanceSize([A class]), class_getInstanceSize([B class]));
    return 0;
}
