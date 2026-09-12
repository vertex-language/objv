// clang's extended vectors, and the overloading the platform's own headers
// are written in.
//
// Neither is in C, and <simd/simd.h> is nothing but the two of them:
//
//	typedef __attribute__((__ext_vector_type__(4))) float simd_float4;
//	static inline SIMD_CFUNC simd_float4 simd_abs(simd_float4 x);
//
// <simd/base.h> refuses to define anything at all unless __has_attribute
// says the compiler has both, and SceneKit, Metal, ModelIO and GameplayKit
// take their geometry in what it defines. A compiler without them cannot
// read <SceneKit/SceneKitTypes.h>, which means it cannot compile a program
// that draws a cube.
//
// What is under test here is the *type* level, which is where a header lives:
// the layout of a vector, its lane names, the elementwise rules, and which
// overload a call picks. Lowering a vector value is a separate thing this
// compiler does not do yet, and refuses to guess at — so nothing here
// evaluates one.
//
// frameworks: Foundation

#import <Foundation/Foundation.h>

typedef __attribute__((__ext_vector_type__(2)))  float f2;
typedef __attribute__((__ext_vector_type__(3)))  float f3;
typedef __attribute__((__ext_vector_type__(4)))  float f4;
typedef __attribute__((__ext_vector_type__(8)))  float f8;
typedef __attribute__((__ext_vector_type__(16))) float f16;
typedef __attribute__((__ext_vector_type__(2)))  double d2;
typedef __attribute__((__ext_vector_type__(3)))  double d3;
typedef __attribute__((__ext_vector_type__(2)))  char c2;
typedef __attribute__((__ext_vector_type__(3)))  char c3;
typedef __attribute__((__ext_vector_type__(16))) char c16;
typedef __attribute__((__ext_vector_type__(32))) char c32;
typedef __attribute__((__ext_vector_type__(1)))  int i1;
typedef __attribute__((__ext_vector_type__(4)))  int i4;
typedef __attribute__((__ext_vector_type__(2)))  long l2;

// The size is not the element count times the element: a machine has no
// three-lane register, so a vector of three floats occupies four of them.
// Every simd header is written against sizeof(simd_float3) == 16.
static void layout(void) {
    printf("%zu %zu %zu %zu %zu\n",
           sizeof(f2), sizeof(f3), sizeof(f4), sizeof(f8), sizeof(f16));
    printf("%zu %zu %zu %zu %zu %zu %zu\n",
           sizeof(d2), sizeof(d3), sizeof(c2), sizeof(c3),
           sizeof(c16), sizeof(c32), sizeof(i1));
    printf("%zu %zu %zu %zu %zu\n",
           _Alignof(f2), _Alignof(f3), _Alignof(f8), _Alignof(c3), _Alignof(c32));
}

// The lane alphabets. A single name yields the element and not a vector of
// one, which is what makes `p.x * p.x + p.y * p.y` ordinary arithmetic; a run
// of them is a swizzle, and a run may repeat and may be longer than the
// alphabet it came from.
typedef __typeof__(((f4 *)0)->x)      LaneOfF4;    // float
typedef __typeof__(((f4 *)0)->xy)     SwizzleTwo;  // float(2)
typedef __typeof__(((f4 *)0)->zyx)    SwizzleThree;
typedef __typeof__(((f4 *)0)->xyzwxyzw) SwizzleEight;
typedef __typeof__(((f4 *)0)->s3)     IndexedLane;
typedef __typeof__(((f4 *)0)->rgba)   ColourFour;
typedef __typeof__(((f4 *)0)->lo)     LowHalf;
typedef __typeof__(((f16 *)0)->sF)    LaneFifteen;

static void lanes(void) {
    printf("%zu %zu %zu %zu %zu %zu %zu %zu\n",
           sizeof(LaneOfF4), sizeof(SwizzleTwo), sizeof(SwizzleThree),
           sizeof(SwizzleEight), sizeof(IndexedLane), sizeof(ColourFour),
           sizeof(LowHalf), sizeof(LaneFifteen));
}

// Elementwise. A scalar operand is spread across the lanes first, which is
// how <simd/common.h> writes `x * 0.5f` on a simd_float4. A comparison
// yields a vector of signed integers as wide as the element — there is no
// vector of bool — and which named type that is matters, because C has two
// names for sixty-four bits and they are not compatible with each other.
typedef __typeof__(*(f4 *)0 + *(f4 *)0)   SumF4;
typedef __typeof__(*(f4 *)0 * 0.5f)       ScaledF4;
typedef __typeof__(*(f4 *)0 == *(f4 *)0)  MaskF4;   // int(4)
typedef __typeof__(*(d2 *)0 < *(d2 *)0)   MaskD2;   // long(2) on LP64
typedef __typeof__(*(c2 *)0 != *(c2 *)0)  MaskC2;
typedef __typeof__(-*(f4 *)0)             NegF4;
typedef __typeof__(~*(i4 *)0)             NotI4;
typedef __typeof__(*(i4 *)0 & *(i4 *)0)   AndI4;

static MaskD2 *maskIsLong(l2 *p) { return (MaskD2 *)p; }   // long(2), not long long(2)
static MaskF4 *maskIsInt(i4 *p)  { return (MaskF4 *)p; }

static void elementwise(void) {
    printf("%zu %zu %zu %zu %zu %zu %zu %zu\n",
           sizeof(SumF4), sizeof(ScaledF4), sizeof(MaskF4), sizeof(MaskD2),
           sizeof(MaskC2), sizeof(NegF4), sizeof(NotI4), sizeof(AndI4));
    printf("%d %d\n", (int)(maskIsLong(0) == 0), (int)(maskIsInt(0) == 0));
}

// ---- overloading ----

// The shape every simd entry point has: one name, one declaration per type,
// and the call decides. The return values are distinct so the program can
// say which one ran.
__attribute__((overloadable)) static int which(f2 x) { (void)x; return 2; }
__attribute__((overloadable)) static int which(f4 x) { (void)x; return 4; }
__attribute__((overloadable)) static int which(d2 x) { (void)x; return 20; }
__attribute__((overloadable)) static int which(int x) { return 100 + x; }
__attribute__((overloadable)) static int which(double x) { return 200 + (int)x; }
__attribute__((overloadable)) static int which(const char *x) { return 300 + (int)strlen(x); }

// Overloading on arity, which is how simd_make_float4 takes one argument or
// four.
__attribute__((overloadable)) static int howMany(int a) { (void)a; return 1; }
__attribute__((overloadable)) static int howMany(int a, int b) { (void)a; (void)b; return 2; }
__attribute__((overloadable)) static int howMany(int a, int b, int c) {
    (void)a; (void)b; (void)c; return 3;
}

// A declaration ahead of the definition, which is how a header is written:
// the whole set is declared first and defined below.
__attribute__((overloadable)) static int ordered(short x);
__attribute__((overloadable)) static int ordered(long x);
__attribute__((overloadable)) static int ordered(short x) { (void)x; return 16; }
__attribute__((overloadable)) static int ordered(long x)  { (void)x; return 64; }

static void overloads(void) {
    printf("%d %d %d\n", which(7), which(1.5), which("abcde"));
    printf("%d %d %d\n", howMany(1), howMany(1, 2), howMany(1, 2, 3));
    printf("%d %d\n", ordered((short)1), ordered(1L));
    // A narrowing that is still a legal assignment picks the arithmetic
    // overload rather than failing.
    char c = 3;
    printf("%d\n", which(c));
}

// Vectors travel through typedefs, pointers, arrays and structs like any
// other type, which is what makes simd_float4x4 a struct of four of them.
typedef struct { f4 columns[4]; } mat4;
typedef struct { f3 v; float w; } mixed;

static void aggregates(void) {
    printf("%zu %zu %zu\n", sizeof(mat4), sizeof(mixed), _Alignof(mat4));
    printf("%zu %zu\n", offsetof(mixed, v), offsetof(mixed, w));
    f4 *p = 0;
    printf("%zu %d\n", sizeof(*p), (int)(p == 0));
}

int main(void) {
    @autoreleasepool {
        layout();
        lanes();
        elementwise();
        overloads();
        aggregates();
    }
    return 0;
}
