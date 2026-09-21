// ARC: a block copied to the heap keeps its captured object alive until the
// block itself goes, past the scope the object was made in.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Res : NSObject
@property (nonatomic) int n;
@end
@implementation Res
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

typedef int (^Getter)(void);

int main(void) {
    Getter g;
    {
        Res *r = [Res new];
        r.n = 7;
        g = [^{ return r.n; } copy];
    }
    printf("scope left\n");
    printf("got %d\n", g());
    g = nil;
    printf("end\n");
    return 0;
}
