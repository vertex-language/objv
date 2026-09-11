// The accessors nobody wrote.
//
// §4.8: a property with neither @synthesize nor @dynamic still has an
// instance variable and a pair of accessors. The analyzer creates the
// declarations, so a send to one typechecks; lower owes the bodies, and a
// class that does not emit them publishes selectors the runtime cannot find.
//
// Most bodies are one load or one store. The interesting ones are a call,
// and which call is the whole of what a property's attributes mean.

@interface Box : NSObject
@property (nonatomic, copy) NSString *name;
@property (nonatomic, retain) NSString *held;
@property (nonatomic, assign) long count;
@property (copy) NSString *shared;
@property (assign) long total;
@property (nonatomic, readonly) long fixed;
@end

@implementation Box
@end

// A nonatomic getter is a load through the ivar's offset — the offset the
// runtime writes when it realizes the class, which is the non-fragile ABI in
// one instruction.
// vir: internal func @__i_Box__name(%self ptr, %_cmd ptr) ptr {
// vir: @_OBJC_IVAR_$_Box._name
// vir: i64.sload32

// A property that owns its value hands it to the runtime, which is what
// makes the old value released and the new one copied or retained.
// vir: call @_objc_setProperty_nonatomic_copy
// vir: call @_objc_setProperty_nonatomic
// vir: call @_objc_setProperty_atomic_copy

// An atomic *object* getter goes through the runtime too: reading a pointer
// and retaining it must not interleave with a setter.
// vir: call @_objc_getProperty

// An atomic scalar is stored directly all the same — a word-sized store is
// already indivisible, and clang emits the same thing.
// vir: internal func @__i_Box__setTotal_(%self ptr, %_cmd ptr, %value i64) {
// vir-not: call @_objc_setProperty_atomic(

// A readonly property gets a getter and no setter.
// vir: internal func @__i_Box__fixed(
// vir-not: @__i_Box__setFixed_

// A hand-written accessor is not replaced: the class wrote it, and what it
// wrote is what runs.
@interface Manual : NSObject
@property (nonatomic, assign) long n;
@end

@implementation Manual
- (long)n { return 42; }
@end
// vir: i32.const 42
// vir: internal func @__i_Manual__setN_(

// A struct-valued property is read through its getter like any other, and a
// member of the result reads from the storage the send wrote into — which is
// how half of AppKit is written. It is not an lvalue, which the analyzer
// refuses; it has an address, which this needs.
struct Rect { double x, y, w, h; };

@interface Framed : NSObject
@property (nonatomic, assign) struct Rect frame;
@end

@implementation Framed
@end

double widthOf(Framed *f) { return f.frame.w; }
// vir: @msgsig_ptr_sret_struct_Rect_ptr_ptr
