// Selectors as values: respondsToSelector, performSelector, and a message
// sent through objc_msgSend with a selector chosen at run time.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/message.h>
#include <stdio.h>

@interface Target : NSObject
- (id)ping;
- (int)add:(int)a to:(int)b;
@end
@implementation Target
- (id)ping { printf("ping\n"); return self; }
- (int)add:(int)a to:(int)b { return a + b; }
@end

int main(void) {
    Target *t = [Target new];
    SEL s = @selector(ping);
    printf("%d %d\n", [t respondsToSelector:s], [t respondsToSelector:@selector(pong)]);
    [t performSelector:s];
    SEL add = NSSelectorFromString(@"add:to:");
    int r = ((int (*)(id, SEL, int, int))objc_msgSend)(t, add, 20, 22);
    printf("%d %s %d\n", r, sel_getName(add), s == @selector(ping));
    return 0;
}
