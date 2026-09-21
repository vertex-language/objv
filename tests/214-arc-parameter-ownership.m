// ARC: an object parameter is a strong variable -- reassigning one in a
// function, a method or a block releases only what the parameter itself took
// -- and a T** parameter points at an __autoreleasing object, filled in by a
// method and by a block and read by the caller after they return.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; }
@end

static int reassign(Obj *o, int times) {
    int seen = o.n;
    for (int i = 0; i < times; i++) {
        o = [Obj new];
        o.n = i;
        seen += o.n;
    }
    return seen;
}

@interface Worker : NSObject
- (int)take:(Obj *)o;
- (BOOL)fail:(int)code error:(NSError **)error;
@end
@implementation Worker
- (int)take:(Obj *)o {
    int n = o.n;
    o = nil;
    return n;
}
- (BOOL)fail:(int)code error:(NSError **)error {
    if (error) *error = [NSError errorWithDomain:@"worker" code:code userInfo:nil];
    return NO;
}
@end

int main(void) {
    @autoreleasepool {
        Obj *keep = [Obj new];
        keep.n = 40;
        printf("%d\n", reassign(keep, 3));
        Worker *w = [Worker new];
        printf("%d %d\n", [w take:keep], keep.n);
        int (^blk)(Obj *) = ^int(Obj *p) { p = [Obj new]; p.n = 2; return p.n; };
        printf("%d %d\n", blk(keep), keep.n);
        NSError *err = nil;
        [w fail:7 error:&err];
        BOOL (^bfail)(NSError **) = ^BOOL(NSError **e) {
            *e = [NSError errorWithDomain:@"block" code:8 userInfo:nil];
            return NO;
        };
        NSError *err2 = nil;
        bfail(&err2);
        printf("%s %ld %s %ld\n", err.domain.UTF8String, (long)err.code, err2.domain.UTF8String,
               (long)err2.code);
    }
    printf("live %d\n", live);
    return 0;
}
