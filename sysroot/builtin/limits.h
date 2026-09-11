/* <limits.h> — C11 §5.2.4.2.1. Values from the target model's
 * predefines; the derived ones (MIN, unsigned MAX) are spelled so
 * they stay correct integer constant expressions usable in #if.
 *
 * The platform's own limits.h comes first and this one has the last
 * word, which is the arrangement clang uses and the only one that
 * works. A hosted <limits.h> carries far more than §5.2.4.2.1 --
 * POSIX's PATH_MAX, OPEN_MAX, and a hundred more -- and a program
 * that includes <limits.h> wants those too. What it must not get is
 * the platform's idea of how wide an int is, since that is the
 * compiler's to state: Darwin's arm/limits.h defines CHAR_BIT itself,
 * and letting it stand means a redefinition warning at best and a
 * disagreement with the type model at worst.
 *
 * So: include theirs, then undefine and restate ours.
 */
#ifndef _OBJV_LIMITS_H
#define _OBJV_LIMITS_H

#if __STDC_HOSTED__ && __has_include_next(<limits.h>)
#include_next <limits.h>
#endif

#undef CHAR_BIT
#undef SCHAR_MAX
#undef SCHAR_MIN
#undef UCHAR_MAX
#undef CHAR_MIN
#undef CHAR_MAX
#undef SHRT_MAX
#undef SHRT_MIN
#undef USHRT_MAX
#undef INT_MAX
#undef INT_MIN
#undef UINT_MAX
#undef LONG_MAX
#undef LONG_MIN
#undef ULONG_MAX
#undef LLONG_MAX
#undef LLONG_MIN
#undef ULLONG_MAX
#undef MB_LEN_MAX

#define CHAR_BIT   __CHAR_BIT__

#define SCHAR_MAX  __SCHAR_MAX__
#define SCHAR_MIN  (-SCHAR_MAX - 1)
#define UCHAR_MAX  (SCHAR_MAX * 2 + 1)

#ifdef __CHAR_UNSIGNED__
#define CHAR_MIN   0
#define CHAR_MAX   UCHAR_MAX
#else
#define CHAR_MIN   SCHAR_MIN
#define CHAR_MAX   SCHAR_MAX
#endif

#define SHRT_MAX   __SHRT_MAX__
#define SHRT_MIN   (-SHRT_MAX - 1)
#define USHRT_MAX  (SHRT_MAX * 2 + 1)

#define INT_MAX    __INT_MAX__
#define INT_MIN    (-INT_MAX - 1)
#define UINT_MAX   (INT_MAX * 2U + 1U)

#define LONG_MAX   __LONG_MAX__
#define LONG_MIN   (-LONG_MAX - 1L)
#define ULONG_MAX  (LONG_MAX * 2UL + 1UL)

#define LLONG_MAX  __LONG_LONG_MAX__
#define LLONG_MIN  (-LLONG_MAX - 1LL)
#define ULLONG_MAX (LLONG_MAX * 2ULL + 1ULL)

/* Bytes in the longest multibyte character: 4 covers UTF-8. */
#define MB_LEN_MAX 4

#endif