// A synthesized scalar property: dot syntax and the accessor messages.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Box : NSObject
@property (nonatomic) int width;
@property (nonatomic) double scale;
@property (nonatomic) BOOL visible;
@end

@implementation Box
@end

int main(void) {
    Box *b = [Box new];
    b.width = 12;
    [b setScale:1.5];
    b.visible = YES;
    b.width += 3;
    printf("%d %g %d\n", b.width, [b scale], b.visible);
    return 0;
}
