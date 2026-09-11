/* <float.h> — C11 §5.2.4.2.2. Pure renaming: every value is a fact
 * about the target's floating formats, computed by cmd/objv from the
 * Model and handed in as a predefine.
 *
 * The platform's own float.h comes first and this one has the last
 * word, for the reason limits.h gives: a hosted header carries more
 * than the standard requires and a program including it wants that
 * too, but how wide a double is belongs to the compiler.
 */
#ifndef _OBJV_FLOAT_H
#define _OBJV_FLOAT_H

#if __STDC_HOSTED__ && __has_include_next(<float.h>)
#include_next <float.h>
#endif

#undef FLT_ROUNDS
#undef FLT_EVAL_METHOD
#undef FLT_RADIX
#undef DECIMAL_DIG
#undef FLT_MANT_DIG
#undef DBL_MANT_DIG
#undef LDBL_MANT_DIG
#undef FLT_DIG
#undef DBL_DIG
#undef LDBL_DIG
#undef FLT_MIN_EXP
#undef DBL_MIN_EXP
#undef LDBL_MIN_EXP
#undef FLT_MIN_10_EXP
#undef DBL_MIN_10_EXP
#undef LDBL_MIN_10_EXP
#undef FLT_MAX_EXP
#undef DBL_MAX_EXP
#undef LDBL_MAX_EXP
#undef FLT_MAX_10_EXP
#undef DBL_MAX_10_EXP
#undef LDBL_MAX_10_EXP
#undef FLT_MAX
#undef DBL_MAX
#undef LDBL_MAX
#undef FLT_EPSILON
#undef DBL_EPSILON
#undef LDBL_EPSILON
#undef FLT_MIN
#undef DBL_MIN
#undef LDBL_MIN
#undef FLT_TRUE_MIN
#undef DBL_TRUE_MIN
#undef LDBL_TRUE_MIN
#undef FLT_HAS_SUBNORM
#undef DBL_HAS_SUBNORM
#undef LDBL_HAS_SUBNORM
#undef FLT_DECIMAL_DIG
#undef DBL_DECIMAL_DIG
#undef LDBL_DECIMAL_DIG

#define FLT_ROUNDS      1
#define FLT_EVAL_METHOD __FLT_EVAL_METHOD__
#define FLT_RADIX       __FLT_RADIX__
#define DECIMAL_DIG     __DECIMAL_DIG__

#define FLT_MANT_DIG    __FLT_MANT_DIG__
#define FLT_DIG         __FLT_DIG__
#define FLT_MIN_EXP     __FLT_MIN_EXP__
#define FLT_MIN_10_EXP  __FLT_MIN_10_EXP__
#define FLT_MAX_EXP     __FLT_MAX_EXP__
#define FLT_MAX_10_EXP  __FLT_MAX_10_EXP__
#define FLT_MAX         __FLT_MAX__
#define FLT_EPSILON     __FLT_EPSILON__
#define FLT_MIN         __FLT_MIN__
#define FLT_TRUE_MIN    __FLT_DENORM_MIN__

#define DBL_MANT_DIG    __DBL_MANT_DIG__
#define DBL_DIG         __DBL_DIG__
#define DBL_MIN_EXP     __DBL_MIN_EXP__
#define DBL_MIN_10_EXP  __DBL_MIN_10_EXP__
#define DBL_MAX_EXP     __DBL_MAX_EXP__
#define DBL_MAX_10_EXP  __DBL_MAX_10_EXP__
#define DBL_MAX         __DBL_MAX__
#define DBL_EPSILON     __DBL_EPSILON__
#define DBL_MIN         __DBL_MIN__
#define DBL_TRUE_MIN    __DBL_DENORM_MIN__

#define LDBL_MANT_DIG   __LDBL_MANT_DIG__
#define LDBL_DIG        __LDBL_DIG__
#define LDBL_MIN_EXP    __LDBL_MIN_EXP__
#define LDBL_MIN_10_EXP __LDBL_MIN_10_EXP__
#define LDBL_MAX_EXP    __LDBL_MAX_EXP__
#define LDBL_MAX_10_EXP __LDBL_MAX_10_EXP__
#define LDBL_MAX        __LDBL_MAX__
#define LDBL_EPSILON    __LDBL_EPSILON__
#define LDBL_MIN        __LDBL_MIN__
#define LDBL_TRUE_MIN   __LDBL_DENORM_MIN__

#endif