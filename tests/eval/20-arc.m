// arc
//
// §7.4's automatic reference counting, which is a second lowering of the
// same language rather than a pass over the first. Everything here is a
// retain or a release the program did not write.

// A __strong local owns what it holds from its declaration to the end of its
// scope, and a store into one lets go of what it replaced.
NSString *shadow(NSString *in);
NSString *shadow(NSString *in) {
    NSString *held = in;
    held = [in copy];
    return held;
}
// vir: call @_objc_retain
// vir: call @_objc_release

// A __weak local is four runtime calls and no stores: the runtime keeps the
// side table, and a read goes through it.
NSString *peek(NSString *strong);
NSString *peek(NSString *strong) {
    __weak NSString *w = strong;
    return w;
}
// vir: call @_objc_initWeak
// vir: call @_objc_loadWeak
// vir: call @_objc_destroyWeak

// §6.5's bridge casts are about ownership and nothing else. __bridge_retained
// hands ARC's reference out, so the result is +1 and the far side owes the
// release; __bridge_transfer takes one in, so the value arrives owned; and
// __bridge moves nothing at all.
typedef const void *OpaqueRef;
OpaqueRef handOut(NSString *s);
OpaqueRef handOut(NSString *s) { return (__bridge_retained OpaqueRef)s; }
NSString *takeIn(OpaqueRef r);
NSString *takeIn(OpaqueRef r) { return (__bridge_transfer NSString *)r; }
NSString *justLook(OpaqueRef r);
NSString *justLook(OpaqueRef r) { return (__bridge NSString *)r; }

// A method in one of the retaining families returns +1; every other returns
// +0, which for a value this frame owns means the autorelease pool.
@interface Factory : NSObject
- (NSString *)newName;
- (NSString *)name;
@end
@implementation Factory
- (NSString *)newName { return [[NSString alloc] init]; }
- (NSString *)name { return [[NSString alloc] init]; }
@end
// vir: call @_objc_autoreleaseReturnValue

// A __block object is owned by its structure, which is what the layout
// nibble says: strong here, unretained without ARC.
void capture(void);
void capture(void) {
    __block NSString *held = 0;
    void (^b)(void) = ^{ held = @"x"; };
    b();
}
// vir: i32.const 838860800
