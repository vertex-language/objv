// ARC: strong ivars are released when their object is deallocated, after
// its own dealloc body runs, and weak ivars are zeroed.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Part : NSObject
@property (nonatomic) int n;
@end
@implementation Part
- (void)dealloc { printf("dealloc part %d\n", _n); }
@end

@interface Whole : NSObject {
@public
    Part *_a;
    Part *_b;
    __weak Part *_w;
    __unsafe_unretained Part *_u;
}
@end
@implementation Whole
- (void)dealloc { printf("dealloc whole\n"); }
@end

int main(void) {
    Part *outside = [Part new];
    outside.n = 3;
    {
        Whole *w = [Whole new];
        w->_a = [Part new]; w->_a.n = 1;
        w->_b = [Part new]; w->_b.n = 2;
        w->_w = outside;
        w->_u = outside;
        w->_a = w->_b;
        printf("leaving\n");
    }
    printf("end %d\n", outside.n);
    return 0;
}
