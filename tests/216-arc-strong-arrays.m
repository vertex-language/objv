// ARC: arrays of object pointers own their elements -- locals of one and two
// dimensions released at scope end, last element first, including when the
// scope is left by break and return; element replacement releasing the old
// one; and static and global arrays holding on past every scope.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
+ (instancetype)newWith:(int)n;
@end
static int live;

@implementation Obj
+ (instancetype)newWith:(int)n { Obj *o = [self new]; o.n = n; return o; }
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc {
    live--;
    if (_n < 40) printf("dealloc %d\n", _n);
}
@end

Obj *globals[2];

static int early(int stop) {
    Obj *arr[3] = { [Obj newWith:30], [Obj newWith:31], [Obj newWith:32] };
    if (stop) return arr[1].n;
    return 0;
}

static Obj *remembered(int n) {
    static Obj *seen[2];
    seen[n % 2] = [Obj newWith:40 + n];
    return seen[0];
}

int main(void) {
    @autoreleasepool {
        {
            Obj *arr[3] = { [Obj newWith:1], [Obj newWith:2] };
            arr[2] = [Obj newWith:3];
            printf("replace\n");
            arr[0] = [Obj newWith:4];
            printf("leaving 1-d\n");
        }
        {
            Obj *grid[2][2];
            for (int i = 0; i < 2; i++)
                for (int j = 0; j < 2; j++) grid[i][j] = [Obj newWith:10 + i * 2 + j];
            printf("leaving 2-d\n");
        }
        for (int i = 0; i < 3; i++) {
            Obj *row[2] = { [Obj newWith:20 + i] };
            if (i == 1) break;
            printf("row %d\n", row[0].n);
        }
        printf("early %d\n", early(1));
        for (int n = 0; n < 4; n++) {
            Obj *first = remembered(n);
            (void)first;
        }
        printf("remembered %d\n", remembered(4).n);
        globals[0] = [Obj newWith:50];
        globals[1] = globals[0];
        globals[0] = nil;
        printf("global %d\n", globals[1].n);
        globals[1] = nil;
    }
    printf("end live %d\n", live);
    return 0;
}
