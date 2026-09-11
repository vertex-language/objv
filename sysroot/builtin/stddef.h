/* <stddef.h> — C11 §7.19. Types from the target model's predefines. */
#ifndef _OBJV_STDDEF_H
#define _OBJV_STDDEF_H

/* The platform's own stddef.h first, when there is one: a hosted header
   carries far more than the standard requires -- Darwin's brings
   rsize_t, errno_t, and the __darwin machinery every other system
   header is written against -- and a program that includes it wants
   those too. What follows fills in only what the platform left out,
   so nothing here contradicts it. */
#if __STDC_HOSTED__ && __has_include_next(<stddef.h>)
#include_next <stddef.h>
#endif

typedef __PTRDIFF_TYPE__ ptrdiff_t;
typedef __SIZE_TYPE__    size_t;
typedef __WCHAR_TYPE__   wchar_t;

/* The type with the strictest alignment. long double has it on every
   model objv currently carries; revisit if a Model gains a stricter one. */
typedef long double max_align_t;

/* §7.19 lets NULL be any implementation-defined null pointer constant, so a
   platform header that got here first has already spelled it its own way —
   __DARWIN_NULL on Apple's SDK. Redefining a macro to a different token
   sequence is a constraint violation, and diagnosing one the user cannot fix
   is noise, so this replaces rather than collides. */
#undef NULL
#define NULL ((void *)0)

/* Constant-folds in the analyzer; no compiler magic required.
 *
 * This one replaces the platform's rather than deferring to it. Apple's
 * <sys/_types/_offsetof.h> defines offsetof as __offsetof, which nothing
 * in the SDK defines: it is written expecting the *compiler's* stddef.h to
 * have supplied one already, and the fallback is dead text. */
#undef offsetof
#define offsetof(type, member) ((size_t)&(((type *)0)->member))

#endif