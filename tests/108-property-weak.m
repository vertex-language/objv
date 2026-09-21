// A weak property becomes nil when its object is deallocated.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Node : NSObject
@property (nonatomic, weak) Node *parent;
@property (nonatomic, strong) Node *child;
@property (nonatomic) int n;
@end
@implementation Node
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

int main(void) {
    Node *child = [Node new];
    child.n = 2;
    @autoreleasepool {
        Node *parent = [Node new];
        parent.n = 1;
        parent.child = child;
        child.parent = parent;
        printf("parent %d\n", child.parent.n);
    }
    printf("parent nil %d\n", child.parent == nil);
    return 0;
}
