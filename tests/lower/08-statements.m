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
