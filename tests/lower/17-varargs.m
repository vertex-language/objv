// §7.16's variadic machinery.
//
// objv's own <stdarg.h> writes the four macros in terms of builtins that
// take the *address* of the list rather than the list itself, which is what
// lets one signature serve whatever shape __builtin_va_list has: a pointer
// on Darwin's AArch64, where every variadic argument gets one stack slot,
// and four fields on SysV x86-64, where they are in two places.

int total(int n, ...) {
    __builtin_va_list ap;
    __builtin_va_start(&ap);
    int t = 0;
    for (int i = 0; i < n; i++) {
        t += *(int *)__builtin_va_arg_ref(&ap, (int *)0);
    }
    __builtin_va_end(&ap);
    return t;
}
// The var-tail is part of the signature and not only of the call: va_start
// needs to know this function has one.
// vir: export func @_total(%n i32, ...) i32
// vir: va_start
// vir: va_arg_ref
// vir: va_end

// The type is named rather than described, because that is what the
// operation wants: the ABI knowledge it needs to advance the list is the
// same knowledge byval demands, and it reads it off the type.
// vir: type @vaarg_i32

int call(void) { return total(3, 1, 2, 3); }
