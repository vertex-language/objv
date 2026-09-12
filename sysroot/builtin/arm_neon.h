/* arm_neon.h -- the ARM NEON intrinsics, as much of them as the platform's
 * own headers reach for.
 *
 * clang ships this header with the compiler rather than with the SDK, because
 * what is in it is the compiler's: every function here is one instruction,
 * and clang emits it from a builtin rather than from a body. objv has no such
 * builtins yet, so the bodies are here, written in the C the intrinsic means.
 * They are correct and they are not fast; nothing in objv lowers a vector to
 * a vector register yet, so speed is not the question this header answers.
 *
 * It exists because <simd/simd.h> needs it. Apple's simd library is the type
 * vocabulary of SceneKit, Metal, ModelIO and GameplayKit, and on this
 * architecture <simd/logic.h> and <simd/math.h> are written directly against
 * these names:
 *
 *	static inline SIMD_CFUNC simd_bool simd_any(simd_char8 x) {
 *	#elif defined __arm64__ || defined __aarch64__
 *	  return vmaxv_u8(x) & 0x80;
 *
 * A compiler that cannot supply them cannot read the header, and a program
 * that cannot read the header cannot say what a cube looks like.
 *
 * The declarations below are the subset <simd/simd.h> reaches for and not the
 * whole of NEON, which is some four thousand functions. One that is missing
 * is missing loudly -- an undeclared name -- rather than quietly.
 */
#ifndef __ARM_NEON_H
#define __ARM_NEON_H

#include <stdint.h>

/* The rounding, square root and fused-multiply-add intrinsics are the C
 * library's functions under other names, and are declared rather than
 * reimplemented: getting round-half-to-even or the NaN rules subtly wrong
 * here would be a wrong answer in a header nobody reads. */
extern float truncf(float), roundf(float), floorf(float), ceilf(float),
    rintf(float), sqrtf(float), fmaf(float, float, float),
    fmaxf(float, float), fminf(float, float);
extern double trunc(double), round(double), floor(double), ceil(double),
    rint(double), sqrt(double), fma(double, double, double),
    fmax(double, double), fmin(double, double);

/* The vector types. A NEON vector is 64 or 128 bits of one element. */
typedef __attribute__((__ext_vector_type__(8))) int8_t int8x8_t;
typedef __attribute__((__ext_vector_type__(16))) int8_t int8x16_t;
typedef __attribute__((__ext_vector_type__(4))) int16_t int16x4_t;
typedef __attribute__((__ext_vector_type__(8))) int16_t int16x8_t;
typedef __attribute__((__ext_vector_type__(2))) int32_t int32x2_t;
typedef __attribute__((__ext_vector_type__(4))) int32_t int32x4_t;
typedef __attribute__((__ext_vector_type__(1))) int64_t int64x1_t;
typedef __attribute__((__ext_vector_type__(2))) int64_t int64x2_t;
typedef __attribute__((__ext_vector_type__(8))) uint8_t uint8x8_t;
typedef __attribute__((__ext_vector_type__(16))) uint8_t uint8x16_t;
typedef __attribute__((__ext_vector_type__(4))) uint16_t uint16x4_t;
typedef __attribute__((__ext_vector_type__(8))) uint16_t uint16x8_t;
typedef __attribute__((__ext_vector_type__(2))) uint32_t uint32x2_t;
typedef __attribute__((__ext_vector_type__(4))) uint32_t uint32x4_t;
typedef __attribute__((__ext_vector_type__(1))) uint64_t uint64x1_t;
typedef __attribute__((__ext_vector_type__(2))) uint64_t uint64x2_t;
typedef __attribute__((__ext_vector_type__(2))) float float32x2_t;
typedef __attribute__((__ext_vector_type__(4))) float float32x4_t;
typedef __attribute__((__ext_vector_type__(1))) double float64x1_t;
typedef __attribute__((__ext_vector_type__(2))) double float64x2_t;

static __inline__ int8x8_t vmax_s8(int8x8_t __a, int8x8_t __b) { union { int8x8_t v; int8_t e[8]; } x = { .v = __a }; union { int8x8_t v; int8_t e[8]; } y = { .v = __b }; union { int8x8_t v; int8_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int8x16_t vmaxq_s8(int8x16_t __a, int8x16_t __b) { union { int8x16_t v; int8_t e[16]; } x = { .v = __a }; union { int8x16_t v; int8_t e[16]; } y = { .v = __b }; union { int8x16_t v; int8_t e[16]; } r; for (int __i = 0; __i < 16; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int16x4_t vmax_s16(int16x4_t __a, int16x4_t __b) { union { int16x4_t v; int16_t e[4]; } x = { .v = __a }; union { int16x4_t v; int16_t e[4]; } y = { .v = __b }; union { int16x4_t v; int16_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int16x8_t vmaxq_s16(int16x8_t __a, int16x8_t __b) { union { int16x8_t v; int16_t e[8]; } x = { .v = __a }; union { int16x8_t v; int16_t e[8]; } y = { .v = __b }; union { int16x8_t v; int16_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int32x2_t vmax_s32(int32x2_t __a, int32x2_t __b) { union { int32x2_t v; int32_t e[2]; } x = { .v = __a }; union { int32x2_t v; int32_t e[2]; } y = { .v = __b }; union { int32x2_t v; int32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int32x4_t vmaxq_s32(int32x4_t __a, int32x4_t __b) { union { int32x4_t v; int32_t e[4]; } x = { .v = __a }; union { int32x4_t v; int32_t e[4]; } y = { .v = __b }; union { int32x4_t v; int32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint8x8_t vmax_u8(uint8x8_t __a, uint8x8_t __b) { union { uint8x8_t v; uint8_t e[8]; } x = { .v = __a }; union { uint8x8_t v; uint8_t e[8]; } y = { .v = __b }; union { uint8x8_t v; uint8_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint8x16_t vmaxq_u8(uint8x16_t __a, uint8x16_t __b) { union { uint8x16_t v; uint8_t e[16]; } x = { .v = __a }; union { uint8x16_t v; uint8_t e[16]; } y = { .v = __b }; union { uint8x16_t v; uint8_t e[16]; } r; for (int __i = 0; __i < 16; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint16x4_t vmax_u16(uint16x4_t __a, uint16x4_t __b) { union { uint16x4_t v; uint16_t e[4]; } x = { .v = __a }; union { uint16x4_t v; uint16_t e[4]; } y = { .v = __b }; union { uint16x4_t v; uint16_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint16x8_t vmaxq_u16(uint16x8_t __a, uint16x8_t __b) { union { uint16x8_t v; uint16_t e[8]; } x = { .v = __a }; union { uint16x8_t v; uint16_t e[8]; } y = { .v = __b }; union { uint16x8_t v; uint16_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint32x2_t vmax_u32(uint32x2_t __a, uint32x2_t __b) { union { uint32x2_t v; uint32_t e[2]; } x = { .v = __a }; union { uint32x2_t v; uint32_t e[2]; } y = { .v = __b }; union { uint32x2_t v; uint32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint32x4_t vmaxq_u32(uint32x4_t __a, uint32x4_t __b) { union { uint32x4_t v; uint32_t e[4]; } x = { .v = __a }; union { uint32x4_t v; uint32_t e[4]; } y = { .v = __b }; union { uint32x4_t v; uint32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] > y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int8x8_t vmin_s8(int8x8_t __a, int8x8_t __b) { union { int8x8_t v; int8_t e[8]; } x = { .v = __a }; union { int8x8_t v; int8_t e[8]; } y = { .v = __b }; union { int8x8_t v; int8_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int8x16_t vminq_s8(int8x16_t __a, int8x16_t __b) { union { int8x16_t v; int8_t e[16]; } x = { .v = __a }; union { int8x16_t v; int8_t e[16]; } y = { .v = __b }; union { int8x16_t v; int8_t e[16]; } r; for (int __i = 0; __i < 16; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int16x4_t vmin_s16(int16x4_t __a, int16x4_t __b) { union { int16x4_t v; int16_t e[4]; } x = { .v = __a }; union { int16x4_t v; int16_t e[4]; } y = { .v = __b }; union { int16x4_t v; int16_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int16x8_t vminq_s16(int16x8_t __a, int16x8_t __b) { union { int16x8_t v; int16_t e[8]; } x = { .v = __a }; union { int16x8_t v; int16_t e[8]; } y = { .v = __b }; union { int16x8_t v; int16_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int32x2_t vmin_s32(int32x2_t __a, int32x2_t __b) { union { int32x2_t v; int32_t e[2]; } x = { .v = __a }; union { int32x2_t v; int32_t e[2]; } y = { .v = __b }; union { int32x2_t v; int32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ int32x4_t vminq_s32(int32x4_t __a, int32x4_t __b) { union { int32x4_t v; int32_t e[4]; } x = { .v = __a }; union { int32x4_t v; int32_t e[4]; } y = { .v = __b }; union { int32x4_t v; int32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint8x8_t vmin_u8(uint8x8_t __a, uint8x8_t __b) { union { uint8x8_t v; uint8_t e[8]; } x = { .v = __a }; union { uint8x8_t v; uint8_t e[8]; } y = { .v = __b }; union { uint8x8_t v; uint8_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint8x16_t vminq_u8(uint8x16_t __a, uint8x16_t __b) { union { uint8x16_t v; uint8_t e[16]; } x = { .v = __a }; union { uint8x16_t v; uint8_t e[16]; } y = { .v = __b }; union { uint8x16_t v; uint8_t e[16]; } r; for (int __i = 0; __i < 16; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint16x4_t vmin_u16(uint16x4_t __a, uint16x4_t __b) { union { uint16x4_t v; uint16_t e[4]; } x = { .v = __a }; union { uint16x4_t v; uint16_t e[4]; } y = { .v = __b }; union { uint16x4_t v; uint16_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint16x8_t vminq_u16(uint16x8_t __a, uint16x8_t __b) { union { uint16x8_t v; uint16_t e[8]; } x = { .v = __a }; union { uint16x8_t v; uint16_t e[8]; } y = { .v = __b }; union { uint16x8_t v; uint16_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint32x2_t vmin_u32(uint32x2_t __a, uint32x2_t __b) { union { uint32x2_t v; uint32_t e[2]; } x = { .v = __a }; union { uint32x2_t v; uint32_t e[2]; } y = { .v = __b }; union { uint32x2_t v; uint32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ uint32x4_t vminq_u32(uint32x4_t __a, uint32x4_t __b) { union { uint32x4_t v; uint32_t e[4]; } x = { .v = __a }; union { uint32x4_t v; uint32_t e[4]; } y = { .v = __b }; union { uint32x4_t v; uint32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] < y.e[__i] ? x.e[__i] : y.e[__i]; return r.v; }
static __inline__ float32x2_t vmaxnm_f32(float32x2_t __a, float32x2_t __b) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } y = { .v = __b }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = fmaxf(x.e[__i], y.e[__i]); return r.v; }
static __inline__ float32x4_t vmaxnmq_f32(float32x4_t __a, float32x4_t __b) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } y = { .v = __b }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = fmaxf(x.e[__i], y.e[__i]); return r.v; }
static __inline__ float64x2_t vmaxnmq_f64(float64x2_t __a, float64x2_t __b) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } y = { .v = __b }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = fmax(x.e[__i], y.e[__i]); return r.v; }
static __inline__ float32x2_t vminnm_f32(float32x2_t __a, float32x2_t __b) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } y = { .v = __b }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = fminf(x.e[__i], y.e[__i]); return r.v; }
static __inline__ float32x4_t vminnmq_f32(float32x4_t __a, float32x4_t __b) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } y = { .v = __b }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = fminf(x.e[__i], y.e[__i]); return r.v; }
static __inline__ float64x2_t vminnmq_f64(float64x2_t __a, float64x2_t __b) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } y = { .v = __b }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = fmin(x.e[__i], y.e[__i]); return r.v; }
static __inline__ int8x8_t vabs_s8(int8x8_t __a) { union { int8x8_t v; int8_t e[8]; } x = { .v = __a }; union { int8x8_t v; int8_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int8x16_t vabsq_s8(int8x16_t __a) { union { int8x16_t v; int8_t e[16]; } x = { .v = __a }; union { int8x16_t v; int8_t e[16]; } r; for (int __i = 0; __i < 16; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int16x4_t vabs_s16(int16x4_t __a) { union { int16x4_t v; int16_t e[4]; } x = { .v = __a }; union { int16x4_t v; int16_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int16x8_t vabsq_s16(int16x8_t __a) { union { int16x8_t v; int16_t e[8]; } x = { .v = __a }; union { int16x8_t v; int16_t e[8]; } r; for (int __i = 0; __i < 8; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int32x2_t vabs_s32(int32x2_t __a) { union { int32x2_t v; int32_t e[2]; } x = { .v = __a }; union { int32x2_t v; int32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int32x4_t vabsq_s32(int32x4_t __a) { union { int32x4_t v; int32_t e[4]; } x = { .v = __a }; union { int32x4_t v; int32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int64x2_t vabsq_s64(int64x2_t __a) { union { int64x2_t v; int64_t e[2]; } x = { .v = __a }; union { int64x2_t v; int64_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = x.e[__i] < 0 ? -x.e[__i] : x.e[__i]; return r.v; }
static __inline__ int8_t vmaxv_s8(int8x8_t __a) { union { int8x8_t v; int8_t e[8]; } x = { .v = __a }; int8_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ int8_t vmaxvq_s8(int8x16_t __a) { union { int8x16_t v; int8_t e[16]; } x = { .v = __a }; int8_t __t = x.e[0]; for (int __i = 1; __i < 16; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ int16_t vmaxv_s16(int16x4_t __a) { union { int16x4_t v; int16_t e[4]; } x = { .v = __a }; int16_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ int16_t vmaxvq_s16(int16x8_t __a) { union { int16x8_t v; int16_t e[8]; } x = { .v = __a }; int16_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ int32_t vmaxv_s32(int32x2_t __a) { union { int32x2_t v; int32_t e[2]; } x = { .v = __a }; int32_t __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ int32_t vmaxvq_s32(int32x4_t __a) { union { int32x4_t v; int32_t e[4]; } x = { .v = __a }; int32_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ uint8_t vmaxv_u8(uint8x8_t __a) { union { uint8x8_t v; uint8_t e[8]; } x = { .v = __a }; uint8_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ uint8_t vmaxvq_u8(uint8x16_t __a) { union { uint8x16_t v; uint8_t e[16]; } x = { .v = __a }; uint8_t __t = x.e[0]; for (int __i = 1; __i < 16; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ uint16_t vmaxv_u16(uint16x4_t __a) { union { uint16x4_t v; uint16_t e[4]; } x = { .v = __a }; uint16_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ uint16_t vmaxvq_u16(uint16x8_t __a) { union { uint16x8_t v; uint16_t e[8]; } x = { .v = __a }; uint16_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ uint32_t vmaxv_u32(uint32x2_t __a) { union { uint32x2_t v; uint32_t e[2]; } x = { .v = __a }; uint32_t __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ uint32_t vmaxvq_u32(uint32x4_t __a) { union { uint32x4_t v; uint32_t e[4]; } x = { .v = __a }; uint32_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ float vmaxv_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; float __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ float vmaxvq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; float __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ double vmaxvq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; double __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] > __t) __t = x.e[__i]; return __t; }
static __inline__ int8_t vminv_s8(int8x8_t __a) { union { int8x8_t v; int8_t e[8]; } x = { .v = __a }; int8_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ int8_t vminvq_s8(int8x16_t __a) { union { int8x16_t v; int8_t e[16]; } x = { .v = __a }; int8_t __t = x.e[0]; for (int __i = 1; __i < 16; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ int16_t vminv_s16(int16x4_t __a) { union { int16x4_t v; int16_t e[4]; } x = { .v = __a }; int16_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ int16_t vminvq_s16(int16x8_t __a) { union { int16x8_t v; int16_t e[8]; } x = { .v = __a }; int16_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ int32_t vminv_s32(int32x2_t __a) { union { int32x2_t v; int32_t e[2]; } x = { .v = __a }; int32_t __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ int32_t vminvq_s32(int32x4_t __a) { union { int32x4_t v; int32_t e[4]; } x = { .v = __a }; int32_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ uint8_t vminv_u8(uint8x8_t __a) { union { uint8x8_t v; uint8_t e[8]; } x = { .v = __a }; uint8_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ uint8_t vminvq_u8(uint8x16_t __a) { union { uint8x16_t v; uint8_t e[16]; } x = { .v = __a }; uint8_t __t = x.e[0]; for (int __i = 1; __i < 16; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ uint16_t vminv_u16(uint16x4_t __a) { union { uint16x4_t v; uint16_t e[4]; } x = { .v = __a }; uint16_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ uint16_t vminvq_u16(uint16x8_t __a) { union { uint16x8_t v; uint16_t e[8]; } x = { .v = __a }; uint16_t __t = x.e[0]; for (int __i = 1; __i < 8; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ uint32_t vminv_u32(uint32x2_t __a) { union { uint32x2_t v; uint32_t e[2]; } x = { .v = __a }; uint32_t __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ uint32_t vminvq_u32(uint32x4_t __a) { union { uint32x4_t v; uint32_t e[4]; } x = { .v = __a }; uint32_t __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ float vminv_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; float __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ float vminvq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; float __t = x.e[0]; for (int __i = 1; __i < 4; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ double vminvq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; double __t = x.e[0]; for (int __i = 1; __i < 2; __i++) if (x.e[__i] < __t) __t = x.e[__i]; return __t; }
static __inline__ int8_t vaddv_s8(int8x8_t __a) { union { int8x8_t v; int8_t e[8]; } x = { .v = __a }; int8_t __t = 0; for (int __i = 0; __i < 8; __i++) __t = (int8_t)(__t + x.e[__i]); return __t; }
static __inline__ int8_t vaddvq_s8(int8x16_t __a) { union { int8x16_t v; int8_t e[16]; } x = { .v = __a }; int8_t __t = 0; for (int __i = 0; __i < 16; __i++) __t = (int8_t)(__t + x.e[__i]); return __t; }
static __inline__ int16_t vaddv_s16(int16x4_t __a) { union { int16x4_t v; int16_t e[4]; } x = { .v = __a }; int16_t __t = 0; for (int __i = 0; __i < 4; __i++) __t = (int16_t)(__t + x.e[__i]); return __t; }
static __inline__ int16_t vaddvq_s16(int16x8_t __a) { union { int16x8_t v; int16_t e[8]; } x = { .v = __a }; int16_t __t = 0; for (int __i = 0; __i < 8; __i++) __t = (int16_t)(__t + x.e[__i]); return __t; }
static __inline__ int32_t vaddv_s32(int32x2_t __a) { union { int32x2_t v; int32_t e[2]; } x = { .v = __a }; int32_t __t = 0; for (int __i = 0; __i < 2; __i++) __t = (int32_t)(__t + x.e[__i]); return __t; }
static __inline__ int32_t vaddvq_s32(int32x4_t __a) { union { int32x4_t v; int32_t e[4]; } x = { .v = __a }; int32_t __t = 0; for (int __i = 0; __i < 4; __i++) __t = (int32_t)(__t + x.e[__i]); return __t; }
static __inline__ uint8_t vaddv_u8(uint8x8_t __a) { union { uint8x8_t v; uint8_t e[8]; } x = { .v = __a }; uint8_t __t = 0; for (int __i = 0; __i < 8; __i++) __t = (uint8_t)(__t + x.e[__i]); return __t; }
static __inline__ uint8_t vaddvq_u8(uint8x16_t __a) { union { uint8x16_t v; uint8_t e[16]; } x = { .v = __a }; uint8_t __t = 0; for (int __i = 0; __i < 16; __i++) __t = (uint8_t)(__t + x.e[__i]); return __t; }
static __inline__ uint16_t vaddv_u16(uint16x4_t __a) { union { uint16x4_t v; uint16_t e[4]; } x = { .v = __a }; uint16_t __t = 0; for (int __i = 0; __i < 4; __i++) __t = (uint16_t)(__t + x.e[__i]); return __t; }
static __inline__ uint16_t vaddvq_u16(uint16x8_t __a) { union { uint16x8_t v; uint16_t e[8]; } x = { .v = __a }; uint16_t __t = 0; for (int __i = 0; __i < 8; __i++) __t = (uint16_t)(__t + x.e[__i]); return __t; }
static __inline__ uint32_t vaddv_u32(uint32x2_t __a) { union { uint32x2_t v; uint32_t e[2]; } x = { .v = __a }; uint32_t __t = 0; for (int __i = 0; __i < 2; __i++) __t = (uint32_t)(__t + x.e[__i]); return __t; }
static __inline__ uint32_t vaddvq_u32(uint32x4_t __a) { union { uint32x4_t v; uint32_t e[4]; } x = { .v = __a }; uint32_t __t = 0; for (int __i = 0; __i < 4; __i++) __t = (uint32_t)(__t + x.e[__i]); return __t; }
static __inline__ float32x2_t vrnd_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = truncf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrndq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = truncf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vrndq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = trunc(x.e[__i]); return r.v; }
static __inline__ float32x2_t vrnda_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = roundf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrndaq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = roundf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vrndaq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = round(x.e[__i]); return r.v; }
static __inline__ float32x2_t vrndm_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = floorf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrndmq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = floorf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vrndmq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = floor(x.e[__i]); return r.v; }
static __inline__ float32x2_t vrndp_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = ceilf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrndpq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = ceilf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vrndpq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = ceil(x.e[__i]); return r.v; }
static __inline__ float32x2_t vrndx_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = rintf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrndxq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = rintf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vrndxq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = rint(x.e[__i]); return r.v; }
static __inline__ float32x2_t vrndn_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = rintf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrndnq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = rintf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vrndnq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = rint(x.e[__i]); return r.v; }
static __inline__ float32x2_t vsqrt_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = sqrtf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vsqrtq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = sqrtf(x.e[__i]); return r.v; }
static __inline__ float64x2_t vsqrtq_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = sqrt(x.e[__i]); return r.v; }
static __inline__ float32x2_t vfma_f32(float32x2_t __a, float32x2_t __b, float32x2_t __c) { union { float32x2_t v; float e[2]; } a = { .v = __a }; union { float32x2_t v; float e[2]; } b = { .v = __b }; union { float32x2_t v; float e[2]; } c = { .v = __c }; union { float32x2_t v; float e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = fmaf(b.e[__i], c.e[__i], a.e[__i]); return r.v; }
static __inline__ float32x4_t vfmaq_f32(float32x4_t __a, float32x4_t __b, float32x4_t __c) { union { float32x4_t v; float e[4]; } a = { .v = __a }; union { float32x4_t v; float e[4]; } b = { .v = __b }; union { float32x4_t v; float e[4]; } c = { .v = __c }; union { float32x4_t v; float e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = fmaf(b.e[__i], c.e[__i], a.e[__i]); return r.v; }
static __inline__ float64x2_t vfmaq_f64(float64x2_t __a, float64x2_t __b, float64x2_t __c) { union { float64x2_t v; double e[2]; } a = { .v = __a }; union { float64x2_t v; double e[2]; } b = { .v = __b }; union { float64x2_t v; double e[2]; } c = { .v = __c }; union { float64x2_t v; double e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = fma(b.e[__i], c.e[__i], a.e[__i]); return r.v; }
static __inline__ int32x2_t vcvtn_s32_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { int32x2_t v; int32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = (int32_t)rintf(x.e[__i]); return r.v; }
static __inline__ uint32x2_t vcvtn_u32_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }; union { uint32x2_t v; uint32_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = (uint32_t)rintf(x.e[__i]); return r.v; }
static __inline__ int32x4_t vcvtnq_s32_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { int32x4_t v; int32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = (int32_t)rintf(x.e[__i]); return r.v; }
static __inline__ uint32x4_t vcvtnq_u32_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }; union { uint32x4_t v; uint32_t e[4]; } r; for (int __i = 0; __i < 4; __i++) r.e[__i] = (uint32_t)rintf(x.e[__i]); return r.v; }
static __inline__ int64x2_t vcvtnq_s64_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { int64x2_t v; int64_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = (int64_t)rint(x.e[__i]); return r.v; }
static __inline__ uint64x2_t vcvtnq_u64_f64(float64x2_t __a) { union { float64x2_t v; double e[2]; } x = { .v = __a }; union { uint64x2_t v; uint64_t e[2]; } r; for (int __i = 0; __i < 2; __i++) r.e[__i] = (uint64_t)rint(x.e[__i]); return r.v; }

/* Reciprocal and reciprocal-square-root estimate, and the Newton step that
 * refines them. The estimate instructions are approximations with an
 * architecturally specified minimum precision; computing the exact value is
 * within that tolerance and is what these do. The step is the formula the
 * architecture defines, not an approximation of it. */
static __inline__ float32x2_t vrecpe_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }, r; for (int __i = 0; __i < 2; __i++) r.e[__i] = 1.0f / x.e[__i]; return r.v; }
static __inline__ float32x4_t vrecpeq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }, r; for (int __i = 0; __i < 4; __i++) r.e[__i] = 1.0f / x.e[__i]; return r.v; }
static __inline__ float32x2_t vrecps_f32(float32x2_t __a, float32x2_t __b) { union { float32x2_t v; float e[2]; } x = { .v = __a }, y = { .v = __b }, r; for (int __i = 0; __i < 2; __i++) r.e[__i] = 2.0f - x.e[__i] * y.e[__i]; return r.v; }
static __inline__ float32x4_t vrecpsq_f32(float32x4_t __a, float32x4_t __b) { union { float32x4_t v; float e[4]; } x = { .v = __a }, y = { .v = __b }, r; for (int __i = 0; __i < 4; __i++) r.e[__i] = 2.0f - x.e[__i] * y.e[__i]; return r.v; }
static __inline__ float32x2_t vrsqrte_f32(float32x2_t __a) { union { float32x2_t v; float e[2]; } x = { .v = __a }, r; for (int __i = 0; __i < 2; __i++) r.e[__i] = 1.0f / sqrtf(x.e[__i]); return r.v; }
static __inline__ float32x4_t vrsqrteq_f32(float32x4_t __a) { union { float32x4_t v; float e[4]; } x = { .v = __a }, r; for (int __i = 0; __i < 4; __i++) r.e[__i] = 1.0f / sqrtf(x.e[__i]); return r.v; }
static __inline__ float32x2_t vrsqrts_f32(float32x2_t __a, float32x2_t __b) { union { float32x2_t v; float e[2]; } x = { .v = __a }, y = { .v = __b }, r; for (int __i = 0; __i < 2; __i++) r.e[__i] = (3.0f - x.e[__i] * y.e[__i]) * 0.5f; return r.v; }
static __inline__ float32x4_t vrsqrtsq_f32(float32x4_t __a, float32x4_t __b) { union { float32x4_t v; float e[4]; } x = { .v = __a }, y = { .v = __b }, r; for (int __i = 0; __i < 4; __i++) r.e[__i] = (3.0f - x.e[__i] * y.e[__i]) * 0.5f; return r.v; }

/* The interleave halves: zip1 takes the low half of each operand and
 * alternates them, zip2 the high half. */
static __inline__ float32x4_t vzip1q_f32(float32x4_t __a, float32x4_t __b) { union { float32x4_t v; float e[4]; } x = { .v = __a }, y = { .v = __b }, r; r.e[0] = x.e[0]; r.e[1] = y.e[0]; r.e[2] = x.e[1]; r.e[3] = y.e[1]; return r.v; }
static __inline__ float32x4_t vzip2q_f32(float32x4_t __a, float32x4_t __b) { union { float32x4_t v; float e[4]; } x = { .v = __a }, y = { .v = __b }, r; r.e[0] = x.e[2]; r.e[1] = y.e[2]; r.e[2] = x.e[3]; r.e[3] = y.e[3]; return r.v; }
static __inline__ float64x2_t vzip1q_f64(float64x2_t __a, float64x2_t __b) { union { float64x2_t v; double e[2]; } x = { .v = __a }, y = { .v = __b }, r; r.e[0] = x.e[0]; r.e[1] = y.e[0]; return r.v; }
static __inline__ float64x2_t vzip2q_f64(float64x2_t __a, float64x2_t __b) { union { float64x2_t v; double e[2]; } x = { .v = __a }, y = { .v = __b }, r; r.e[0] = x.e[1]; r.e[1] = y.e[1]; return r.v; }

#endif /* __ARM_NEON_H */
