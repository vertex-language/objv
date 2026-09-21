// NSCopying and NSMutableCopying: a custom copy, and copy vs mutableCopy of
// Foundation's collections, which are shallow.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Doc : NSObject <NSCopying>
@property (nonatomic, copy) NSString *title;
@property (nonatomic, strong) NSMutableArray *pages;
@end
@implementation Doc
- (id)copyWithZone:(NSZone *)z {
    Doc *d = [Doc new];
    d.title = self.title;
    d.pages = [self.pages mutableCopy];
    return d;
}
@end

int main(void) {
    @autoreleasepool {
        Doc *a = [Doc new];
        a.title = @"a";
        a.pages = [NSMutableArray arrayWithObjects:@"p1", nil];
        Doc *b = [a copy];
        [b.pages addObject:@"p2"];
        b.title = @"b";
        printf("%s %lu / %s %lu\n", a.title.UTF8String, (unsigned long)a.pages.count,
               b.title.UTF8String, (unsigned long)b.pages.count);
        NSArray *imm = @[ [NSMutableString stringWithString:@"x"] ];
        NSMutableArray *mut = [imm mutableCopy];
        [mut[0] appendString:@"y"];
        [mut addObject:@"z"];
        printf("%s %lu %lu\n", [imm[0] UTF8String], (unsigned long)imm.count, (unsigned long)mut.count);
        printf("%d\n", [imm copy] == imm);
    }
    return 0;
}
