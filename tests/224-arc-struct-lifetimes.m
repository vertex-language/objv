// ARC: C structs that own objects, in every place a struct goes -- passed by
// value (the callee owns and destroys its copy), assigned (retaining the new,
// releasing the old, and surviving `a = a`), nested with an array of
// objects, holding a __weak member that zeroes, returned and discarded or
// used only for a member, declared in a loop, and built as a compound
// literal argument.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@property (nonatomic) int n;
+ (instancetype)newWith:(int)n;
@end
@implementation Obj
+ (instancetype)newWith:(int)n { Obj *o = [self new]; o.n = n; return o; }
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; printf("dealloc %d\n", _n); }
@end

typedef struct { int tag; Obj *obj; } Slot;
typedef struct { Slot main; Obj *extra[2]; __weak Obj *watch; } Nest;

static int peek(Slot s) { return s.obj.n + s.tag; }
static Slot make(int n) { Slot s = { n, [Obj newWith:n] }; return s; }
static Nest build(int n) {
    Nest x = { make(n), { [Obj newWith:n + 1], [Obj newWith:n + 2] }, nil };
    return x;
}

int main(void) {
    {
        Slot a = make(1);
        printf("peek %d\n", peek(a));
        Slot b = a;
        b = b;
        b = make(2);
        printf("a %d b %d\n", a.obj.n, b.obj.n);
    }
    printf("after slots live %d\n", live);

    printf("member %d\n", make(3).obj.n);
    make(4);
    printf("peek literal %d\n", peek((Slot){ 5, [Obj newWith:5] }));
    printf("after temporaries live %d\n", live);

    Obj *watched = [Obj newWith:50];
    {
        Nest n = build(10);
        n.watch = watched;
        n.extra[1] = n.extra[0];
        printf("nest %d %d %d %d\n", n.main.obj.n, n.extra[0].n, n.extra[1].n, n.watch.n);
    }
    printf("after nest live %d\n", live);
    Nest w = { { 0, nil }, { nil, nil }, watched };
    watched = nil;
    printf("watch zeroed %d\n", w.watch == nil);

    for (int i = 0; i < 3; i++) {
        Slot s = make(100 + i);
        if (i == 1) continue;
        printf("loop %d\n", s.obj.n);
    }
    printf("end live %d\n", live);
    return 0;
}
