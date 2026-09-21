// Instance variable layout as the runtime reports it: a root class of the
// program's own that declares its isa, a subclass of NSObject with no ivars,
// ivars from the interface, a class extension and the implementation, of
// struct, double and char type, three levels deep. Every offset and size has
// to be clang's, or a subclass compiled by one and a superclass by the other
// disagree about where the fields are.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>

__attribute__((objc_root_class))
@interface Root {
    Class isa;
    int _rootCount;
}
@end
@implementation Root
+ (void)initialize {}
@end

@interface Empty : NSObject
@end
@implementation Empty
@end

typedef struct { char tag; double weight; } Payload;

@interface Base : NSObject {
    char _c;
    Payload _payload;
}
@end
@interface Base () {
    short _ext;
}
@end
@implementation Base {
    double _impl;
}
@end

@interface Middle : Base {
    char _m;
}
@end
@implementation Middle
@end

@interface Leaf : Middle {
    long _leaf;
    char _last;
}
@end
@implementation Leaf
@end

static void show(Class k) {
    printf("%s size %zu:", class_getName(k), class_getInstanceSize(k));
    unsigned n = 0;
    Ivar *ivars = class_copyIvarList(k, &n);
    for (unsigned i = 0; i < n; i++)
        printf(" %s@%td(%s)", ivar_getName(ivars[i]), ivar_getOffset(ivars[i]),
               ivar_getTypeEncoding(ivars[i]));
    free(ivars);
    printf("\n");
}

int main(void) {
    show(objc_getClass("Root"));
    show([Empty class]);
    show([Base class]);
    show([Middle class]);
    show([Leaf class]);
    return 0;
}
