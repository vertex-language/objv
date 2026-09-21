// ARC: assigning a strong variable or property its own value keeps the
// object alive (retain before release).
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

@interface Holder : NSObject
@property (nonatomic, strong) Obj *obj;
@property (nonatomic, copy) NSString *s;
@end
@implementation Holder
@end

int main(void) {
    Holder *h = [Holder new];
    h.obj = [Obj new];
    h.obj.n = 1;
    h.obj = h.obj;
    printf("still %d\n", h.obj.n);
    Obj *x = [Obj new];
    x.n = 2;
    x = x;
    printf("still %d\n", x.n);
    h.s = [NSMutableString stringWithString:@"self"];
    h.s = h.s;
    printf("%s\n", h.s.UTF8String);
    h = nil;
    printf("end\n");
    return 0;
}
