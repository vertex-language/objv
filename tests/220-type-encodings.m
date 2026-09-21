// The type encodings the runtime hands out, each in the form clang writes
// it: an ivar's names every field of a struct stored by value and the class
// of each object in it, but not behind a pointer; a property's T attribute
// keeps only a top-level object's class; @encode and method types are the
// plain form. Unions, bit-fields and nested structs included.
// mode: mrr
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>

typedef struct { char tag; struct { int a; } inner; int bits : 3; id obj; NSString *s; } P;
typedef union { int i; float f; struct { short lo, hi; } parts; } U;

@interface C : NSObject {
    P _p;
    P *_pp;
    U _u;
    NSString *_str;
    id<NSCopying> _copyable;
}
@property (nonatomic) P prop;
@property (nonatomic, retain) NSArray *list;
@property (nonatomic) U onion;
- (P)take:(P)p ptr:(U *)u;
@end
@implementation C
- (P)take:(P)p ptr:(U *)u { return p; }
@end

int main(void) {
    unsigned n = 0;
    Ivar *iv = class_copyIvarList([C class], &n);
    for (unsigned i = 0; i < n; i++) printf("%s %s\n", ivar_getName(iv[i]), ivar_getTypeEncoding(iv[i]));
    free(iv);
    const char *props[] = { "prop", "list", "onion" };
    for (int i = 0; i < 3; i++)
        printf("%s %s\n", props[i], property_getAttributes(class_getProperty([C class], props[i])));
    printf("%s\n", method_getTypeEncoding(class_getInstanceMethod([C class], @selector(take:ptr:))));
    printf("%s | %s\n", @encode(P), @encode(U));
    return 0;
}
