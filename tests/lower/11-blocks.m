// Blocks: the structure, the function, the descriptor, and the helpers.
//
// Every number below is one clang writes too, read out of an -S listing:
// the flags word, the descriptor's size, and the @encode signature. They
// are the block ABI and not this compiler's choices, which is why they are
// asserted here rather than described.

typedef int (^Adder)(int);
typedef void (^Action)(void);

// A literal that captured nothing is a *global* block: the whole structure
// is constant, because nothing about it differs between two executions of
// the statement that wrote it. BLOCK_IS_GLOBAL | BLOCK_HAS_SIGNATURE.
int twice(int n) {
    Adder d = ^(int x) { return x * 2; };
    return d(n);
}
// vir: internal global ro @___block_literal_global
// vir: @__NSConcreteGlobalBlock
// vir: 1342177280
// vir: internal func @___twice_block_invoke(%block ptr, %x i32) i32
// The signature the descriptor carries: an int returned, a twelve-byte
// frame, the block at 0 and the int at 8.
// vir: "i12@?0i8"

// A literal that captured something is a *stack* block, built by stores into
// the frame. BLOCK_HAS_SIGNATURE alone: an int needs nothing retained.
int adds(int n) {
    Adder d = ^(int x) { return x + n; };
    return d(1);
}
// vir: internal func @___adds_block_invoke
// vir: 1073741824
// The header is 32 bytes, so the first capture is at 32 and the literal is
// 36 — not rounded up, which is what the runtime copies.
// vir: ptr.alloc 36
// vir: 0, 36,

// Calling one is `b->invoke(b, args…)`, and invoke is at 16 in every block
// there has ever been.
// vir: i64.const 16
// vir: callind

// A capture the runtime has to keep alive brings the two helpers with it,
// and BLOCK_HAS_COPY_DISPOSE with them: 0x42000000.
Action hold(NSString *s) {
    return ^{ [s length]; };
}
// vir: internal func @___copy_helper_block_
// vir: internal func @___destroy_helper_block_
// vir: call @__Block_object_assign
// vir: call @__Block_object_dispose
// vir: 1107296256
// BLOCK_FIELD_IS_OBJECT, which is what the helpers say they are handling.
// vir: i32.const 3

// Two literals in one function are numbered the way clang numbers them, so
// that a backtrace reads the same.
int both(int n) {
    Adder a = ^(int x) { return x + n; };
    Adder b = ^(int x) { return x - n; };
    return a(1) + b(2);
}
// vir: @___both_block_invoke(
// vir: @___both_block_invoke_2(

// A block in a method reaches an instance variable through the self it
// captured: §4.5's bare names are as visible inside a block as outside one.
@interface Counter : NSObject { int _n; }
- (int)viaBlock;
@end

@implementation Counter
- (int)viaBlock {
    Adder d = ^(int x) { return x + _n; };
    return d(1);
}
@end
// vir: %self_addr = ptr.add %block
