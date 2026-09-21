// readonly properties, renamed getters and setters, and a custom accessor
// that replaces the synthesized one.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Light : NSObject
@property (nonatomic, getter=isOn, setter=turn:) BOOL on;
@property (nonatomic, readonly) int switches;
@property (nonatomic) int level;
@end

@implementation Light {
    int _raw;
}
@synthesize level = _raw;
- (void)turn:(BOOL)on { _on = on; _switches++; }
- (int)level { return _raw * 10; }
@end

int main(void) {
    Light *l = [Light new];
    l.on = YES;
    [l turn:NO];
    l.on = YES;
    l.level = 4;
    printf("%d %d %d\n", l.isOn, l.switches, l.level);
    return 0;
}
