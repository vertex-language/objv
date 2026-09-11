// The Objective-C statements (§7).

void pooled(id x) {
    @autoreleasepool {
        [x copy];
    }
}
// vir: import func @_objc_autoreleasePoolPush() ptr
// vir: @_objc_autoreleasePoolPop

void locked(id lock, id x) {
    @synchronized (lock) {
        [x copy];
    }
}
// vir: @_objc_sync_enter
// vir: @_objc_sync_exit

int walk(NSArray *a) {
    int n = 0;
    for (id x in a) {
        if (x == 0) continue;
        n++;
    }
    return n;
}
// Two nested loops, because the collection hands over a batch at a time.
// vir: "countByEnumeratingWithState:objects:count:"
// vir: @forin_batch
// vir: @forin_inner
// The mutation check is not optional: a collection modified mid-walk would
// otherwise be enumerated through a buffer that no longer describes it.
// vir: @_objc_enumerationMutation

void raise(id e) { @throw e; }
// objc_exception_throw does not return, and the block has to end somewhere.
// vir: @_objc_exception_throw
// vir: trap

// §6.8.1 puts a label on a statement, and the statement may be another
// label: `case 1: case 2: return x;` is two labels on one return. Each is a
// target the switch's branch chain names, so each opens a block, and the
// blocks fall through to the one holding the statement.
int stacked(int c) {
    switch (c) {
    case 1: case 2: case 3: return 1;
    case 4: return 2;
    }
    return 0;
}
// vir: @switch_case

// A label inside a loop inside the switch is Duff's device, and it is the
// reason the blocks are found by lookup where the label stands rather than
// walked for at the top of the body.
void duff(char *to, const char *from, int count) {
    int n = (count + 7) / 8;
    switch (count % 8) {
    case 0: do { *to++ = *from++;
    case 7:      *to++ = *from++;
    case 6:      *to++ = *from++;
            } while (--n > 0);
    }
}
// vir: @do_body

// GCC's binary conditional, which is everywhere in Objective-C: the value is
// read once and yielded when it is true. `a ? a : b` would read it twice, and
// a is usually a call.
NSString *orDefault(NSString *name);
NSString *orDefault(NSString *name) { return name ?: @"untitled"; }
// vir: cond_then

// §6.3.2.1p3: an array operand of a binary operator is a pointer to its
// first element. Asking the declared type instead says it is arithmetic on
// an array, which has no lowering.
char *third(char *s);
char *third(char *s) {
    char local[8] = "abcdefg";
    (void)local;
    return s + 2;
}
long span(void);
long span(void) {
    int cells[4] = {1, 2, 3, 4};
    return (cells + 3) - cells;
}
// vir: ptr.add
