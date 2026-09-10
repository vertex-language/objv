// Message sends (§6.3), and the references the runtime rewrites.
//
// A send is a call through a pointer to objc_msgSend, typed with the call
// site's own signature: one trampoline has as many signatures as there are
// methods, and a direct call would fix one on the imported symbol and
// mis-call every other method through it.

@interface Counter : NSObject {
    int _count;
}
- (int)count;
- (void)add:(int)n;
+ (Counter *)zero;
@end

@implementation Counter
- (int)count { return _count; }
// An instance variable's offset is a global the runtime writes: that is the
// non-fragile ABI in one load.
// vir: ptr.getaddr @_OBJC_IVAR_$_Counter$_count
// vir: i64.sload32

- (void)add:(int)n { _count = _count + n; }

+ (Counter *)zero { return [[Counter alloc] init]; }
// vir: @_OBJC_CLASSLIST_REFERENCES_$_

- (int)twiceCount { return [self count] * 2; }
// A super send goes through objc_msgSendSuper2 and a two-word structure of
// the receiver and the class the method was compiled in.
- (void)dealloc { [super dealloc]; }
// vir: @_objc_msgSendSuper2
// vir: __DATA,__objc_superrefs
@end

int use(Counter *c) {
    [c add:2];
    return [c count];
}
// vir: import func @_objc_msgSend(ptr, ptr, ...) ptr
// vir: callind
// vir: @_OBJC_SELECTOR_REFERENCES_
// vir: __DATA,__objc_selrefs,literal_pointers,no_dead_strip

SEL which(void) { return @selector(add:); }
// vir: "add:"

const char *encoding(void) { return @encode(int); }
// vir: = "i"
