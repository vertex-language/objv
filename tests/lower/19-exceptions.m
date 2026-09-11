// §7.2's @try, @catch, @finally and @throw.

// The whole of it in one function: a call inside the body carries an unwind
// edge, the pad names one clause per @catch, and the personality's selector
// says which one matched.
void risky(void);

int caught(void) {
    int r = 0;
    @try {
        risky();
        r = 1;
    } @catch (NSException *e) {
        r = 2;
    }
    return r;
}
// vir: personality @___objc_personality_v0
// vir: invoke @_risky() to
// vir: unwind @try
// vir: pad (%exn ptr, %sel i32) catch @_OBJC_EHTYPE_$_NSException
// vir: call @_objc_begin_catch(%exn)
// vir: call @_objc_end_catch()

// A clause that names no class catches everything, which the pad spells as a
// clause with no type-info at all.
int anything(void) {
    @try {
        risky();
    } @catch (...) {
        return 1;
    }
    return 0;
}
// vir: catch null

// `id` is a class in the table rather than the absence of one: the runtime
// publishes a type-info that matches any object.
int anyObject(void) {
    @try {
        risky();
    } @catch (id e) {
        (void)e;
        return 1;
    }
    return 0;
}
// vir: catch @_OBJC_EHTYPE_id

// A @finally runs on every way out, so it is one block reached from all of
// them with a slot saying where to go next -- and the unwinding way out is a
// catch-all clause and a rethrow, not a cleanup and a resume, because a
// resume would leave the function and an enclosing @try would never see it.
int cleanly(int n) {
    @try {
        risky();
        return n;
    } @finally {
        risky();
    }
}
// vir: call @_objc_exception_rethrow()
// vir: br_table

// @throw is a call like any other, and one inside a @try is an invoke: the
// clauses of the very @try it stands in are entitled to see it.
void raises(id obj) {
    @try {
        @throw obj;
    } @catch (id e) {
        @throw;
    }
}
// vir: invoke @_objc_exception_throw
// vir: @_objc_exception_rethrow
