// A strong (retain) object property keeps its value alive; replacing it
// releases the old one.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Tag : NSObject
@property (nonatomic) int n;
@end
@implementation Tag
- (void)dealloc {
    printf("dealloc tag %d\n", _n);
#if !__has_feature(objc_arc)
    [super dealloc];
#endif
}
@end

@interface Holder : NSObject
@property (nonatomic, strong) Tag *tag;
@end
@implementation Holder
@end

static Tag *make(int n) {
    Tag *t = [Tag new];
    t.n = n;
#if __has_feature(objc_arc)
    return t;
#else
    return [t autorelease];
#endif
}

int main(void) {
    Holder *h = [Holder new];
    @autoreleasepool {
        h.tag = make(1);
    }
    printf("holding %d\n", h.tag.n);
    @autoreleasepool {
        h.tag = make(2);
        printf("replaced\n");
    }
    printf("holding %d\n", h.tag.n);
    return 0;
}
