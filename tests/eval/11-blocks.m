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

// __block: one variable, shared by the function and by every block that
// captured it, and still shared after the block has outlived the frame.
//
// It lives in a structure of its own rather than in the frame, and every
// access goes through that structure's forwarding field — which is what
// makes it keep working when _Block_copy moves the structure to the heap
// and rewrites the stack copy's forwarding to point at it.
int shared(void) {
    __block int n = 5;
    Action bump = ^{ n++; };
    bump();
    return n;
}
// The header is 24 bytes and an int lands at 24, so the structure is 32 --
// rounded up to a pointer, which is what clang writes into the size field.
// vir: %n_byref = ptr.alloc 32 align 8
// vir: i32.const 32
// The literal captures the structure's address, and the copy helper hands
// it to the runtime with BLOCK_FIELD_IS_BYREF.
// vir: i32.const 8
// Every access is a load of forwarding and then the variable's offset.
// vir: i64.const 8
// vir: i64.const 24

// A __block object carries the structure's own helpers, and the flags say
// so: BLOCK_BYREF_LAYOUT_UNRETAINED | BLOCK_BYREF_HAS_COPY_DISPOSE, which
// is 0x52000000.
NSString *held(NSString *s) {
    __block NSString *box = s;
    Action keep = ^{ [box length]; };
    keep();
    return box;
}
// vir: i32.const 1375731712
// vir: @___Block_byref_object_copy_
// vir: @___Block_byref_object_dispose_
// BLOCK_FIELD_IS_OBJECT | BLOCK_BYREF_CALLER, which is what tells the
// runtime the caller is the byref machinery.
// vir: i32.const 131

// An aggregate is captured by copying its bytes into the literal, which is
// what "by value" means for something no register holds. A __block one lives
// in its structure the same way, and the block reaches it through forwarding
// like any other.
struct Pt { double x, y; };
double moved(void) {
    struct Pt p = { 1, 2 };
    __block struct Pt q = { 10, 20 };
    Action shift = ^{ q.x += p.x; q.y += p.y; };
    shift();
    return q.x + q.y;
}
// vir: memcpy
// vir: %q_byref = ptr.alloc

// A block body sees file scope and its own names, and not the enclosing
// function's locals — but a `static` local is not a local: §6.2.4 gives it
// static storage duration, it lives where a global lives, and a block that
// names one is naming a global. The binding is written inside the function
// all the same, so it has to come with it.
int counted(void);
int counted(void) {
    static int calls = 0;
    enum { step = 3 };
    void (^bump)(void) = ^{ calls += step; };
    bump();
    return calls;
}
// vir: @_static.calls
