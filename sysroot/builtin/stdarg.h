/* <stdarg.h> — C11 §7.16.
 *
 * The four macros are the C spelling of VIR's va_start, va_arg, va_end
 * and va_copy. Each takes the *address* of the list rather than the
 * list itself, which is what lets one signature serve whatever shape
 * __builtin_va_list has on the target: a pointer on Darwin's AArch64,
 * where every variadic argument gets one stack slot, and four fields on
 * SysV x86-64, where they are in a register save area and the caller's
 * outgoing area both.
 *
 * va_arg's second argument is a null pointer that exists only to carry
 * the type. C has no other way to hand a type to something that is not
 * a keyword, and the operation underneath wants one — it advances the
 * list by that type's size and yields its address, which the macro
 * dereferences.
 *
 * va_start's `last` is dropped. It names the last fixed parameter,
 * which mattered to a compiler that computed the tail's address from
 * it; the operation underneath knows where this function's own
 * parameters ended. It is still written by every program, and still
 * has to parse, so the macro takes it and evaluates nothing of it.
 *
 * Nothing in Objective-C's own surface needs any of this: a message
 * send is a call and not a variadic one, and objc_msgSend's var-tail is
 * the caller's arguments rather than a va_list the program walks. What
 * needs them is the C a program is written in beside it.
 */
#ifndef _OBJV_STDARG_H
#define _OBJV_STDARG_H

/* The compiler's own type, whose shape is the target's. Apple's
   <sys/_types/_va_list.h> writes the same typedef, so the two agree
   wherever both are included. */
typedef __builtin_va_list va_list;

#define va_start(ap, last) __builtin_va_start(&(ap))
#define va_arg(ap, type) (*(type *)__builtin_va_arg_ref(&(ap), (type *)0))
#define va_end(ap) __builtin_va_end(&(ap))
#define va_copy(dst, src) __builtin_va_copy(&(dst), &(src))

#endif