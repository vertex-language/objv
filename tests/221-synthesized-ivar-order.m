// Where synthesized instance variables go: after every declared one --
// interface, extension, implementation, in the order written -- and among
// themselves smallest first, as clang lays them out to save padding. The
// order is the layout, so it has to be clang's for code built by the two to
// share a class.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>
@interface A : NSObject { int _explicit; }
@property (nonatomic) char c1;
@property (nonatomic) double d1;
@property (nonatomic) char c2;
@property (nonatomic, retain) id o1;
@property (nonatomic) short s1;
@end
@interface A () 
@property (nonatomic) long extProp;
@end
@implementation A { char _implIvar; }
@synthesize s1 = _custom;
@end
@interface B : NSObject
@property (nonatomic) int x;
@property (nonatomic) int y;
@property (nonatomic) int z;
@end
@implementation B
@end
static void show(Class k) {
    unsigned n = 0; Ivar *iv = class_copyIvarList(k, &n);
    for (unsigned i = 0; i < n; i++) printf("%s@%td ", ivar_getName(iv[i]), ivar_getOffset(iv[i]));
    printf("size %zu\n", class_getInstanceSize(k)); free(iv);
}
int main(void) { show([A class]); show([B class]); return 0; }
