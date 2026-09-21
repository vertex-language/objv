// Class properties: @property (class), backed by a static the class owns.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Config : NSObject
@property (class, nonatomic) int level;
@property (class, nonatomic, readonly) const char *name;
@end

static int gLevel = 3;

@implementation Config
+ (int)level { return gLevel; }
+ (void)setLevel:(int)level { gLevel = level; }
+ (const char *)name { return "config"; }
@end

int main(void) {
    printf("%d %s\n", Config.level, Config.name);
    Config.level = 9;
    Config.level += 1;
    printf("%d %d\n", [Config level], gLevel);
    return 0;
}
