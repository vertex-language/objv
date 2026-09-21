// ARC: objects chosen by ?:, by ?: with nil, and by a statement expression
// are retained once and released once.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@end
@implementation Obj
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; }
@end

int main(void) {
    @autoreleasepool {
        Obj *a = [Obj new], *b = [Obj new];
        for (int i = 0; i < 4; i++) {
            Obj *pick = i % 2 ? a : b;
            Obj *made = i < 2 ? [Obj new] : nil;
            Obj *orNew = made ?: [Obj new];
            Obj *se = ({ Obj *t = [Obj new]; t; });
            (void)pick; (void)orNew; (void)se;
        }
        printf("live in pool %d\n", live);
    }
    printf("live after %d\n", live);
    return 0;
}
