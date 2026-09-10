/* <stdarg.h> — C11 §7.16.
 *
 * This is the classic stack-walk implementation: va_list is a byte
 * pointer and va_arg steps it by rounded argument sizes. It parses
 * and analyzes cleanly, which is what the front end needs today.
 *
 * It is NOT correct for register-passing ABIs (x86-64 SysV passes the
 * first six integer arguments in registers, and AAPCS64 the first
 * eight). The four macros below are the C spelling of VIR's va_start,
 * va_arg, va_end and va_copy, and this file is where the real
 * semantics land when lower implements them — same file, same names.
 *
 * Nothing in Objective-C's own surface needs them: a message send is a
 * call and not a variadic one, and objc_msgSend's var-tail is the
 * caller's arguments rather than a va_list the program walks. What
 * needs them is the C a program is written in beside it, and what
 * needs them first is NSLog.
 */
#ifndef _OBJV_STDARG_H
#define _OBJV_STDARG_H

typedef void *va_list;

#define va_start(ap, last) __builtin_va_start(&(ap))
#define va_arg(ap, type) (*(type *)__builtin_va_arg_ref(&(ap), (type *)0))
#define va_end(ap) __builtin_va_end(&(ap))
#define va_copy(dst, src) __builtin_va_copy(&(dst), &(src))

#endif