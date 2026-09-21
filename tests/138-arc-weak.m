// ARC: a __weak local is zeroed when its object goes, and loading it yields
// a strong reference that keeps the object alive while used.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@end
@implementation Obj
- (void)dealloc { printf("dealloc\n"); }
@end

int main(void) {
    __weak Obj *w;
    {
        Obj *s = [Obj new];
        w = s;
        printf("alive %d\n", w != nil);
        Obj *again = w;
        s = nil;
        printf("still alive %d\n", w != nil);
        (void)again;
    }
    printf("zeroed %d\n", w == nil);
    __weak id never = nil;
    printf("%d\n", never == nil);
    return 0;
}
