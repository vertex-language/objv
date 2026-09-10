// The shapes real headers are written in.
//
// Every construct here was found by preprocessing
// <Foundation/Foundation.h> and checking the result: each one is something
// Apple's headers do that objv rejected, and each one clang accepts. They
// are together in one file because they have nothing in common except
// where they came from.

// §4.1 admits an attribute sequence before @interface. Apple's root class
// is declared behind an export macro, so what precedes the '@' is a whole
// declaration-specifier list — OBJC_EXPORT is
// `extern __attribute__((visibility("default")))` — and the storage class
// is dropped, a class having no linkage of its own to give one to.
__attribute__((objc_root_class))
extern __attribute__((visibility("default")))
@interface Root
- (instancetype)init;
@end

// gcc's double-underscore spellings of the C keywords. Foundation's
// transitive closure writes `extern __inline __attribute__((__gnu_inline__))`
// more than a hundred times.
extern __inline __attribute__((__gnu_inline__)) int twice(int x) { return x * 2; }
static __const__ int limit = 10;
static __volatile__ int flag;

// __builtin_va_list is the compiler's, not a header's:
// <sys/_types/_va_list.h> typedefs va_list from it and declares it nowhere.
typedef __builtin_va_list va_list;
extern void logv(const char *fmt, va_list args);

// (`#if !defined(__unsafe_unretained)`, which NSObjCRuntime.h writes and
// whose operand is a keyword to phase 7 and an identifier to phase 4, is a
// preprocessor question and is tested there.)

// NS_ENUM with a fixed underlying type writes an opaque enum declaration
// and then the definition. The first completes the type; only the second
// defines the enumeration, so the second is not a redefinition of it.
typedef long NSInteger;
typedef unsigned long NSUInteger;
typedef enum __attribute__((enum_extensibility(closed))) NSComparisonResult : NSInteger NSComparisonResult;
enum NSComparisonResult : NSInteger {
    NSOrderedAscending = -1L,
    NSOrderedSame,
    NSOrderedDescending
};

// An enumerator whose top bit is set is representable in the unsigned type
// it was declared with, and ~0u is UINT_MAX rather than -1: the value has
// the operand's type and width, not the folder's.
typedef enum : unsigned long long {
    NSAlignMinXInward   = 1ULL << 0,
    NSAlignRectFlipped  = 1ULL << 63
} NSAlignmentOptions;

enum ipc_object_type : unsigned int {
    IPC_OTYPE_KCDATA  = 52,
    IPC_OTYPE_UNKNOWN = ~0u
};

@interface Shapes : Root {
    unsigned int _refs : 16;
    // A flexible array member may be the last instance variable, as it may
    // be the last member of a struct. NSDecimal's _mantissa is one.
    unsigned short _mantissa[];
}

// A class property and an instance property may share a name: they are
// reached through different selectors on different objects. NSDate and
// NSThread both do this.
@property (readonly) NSInteger interval;
@property (class, readonly) NSInteger interval;

// A property declares its accessors, and a class may declare either
// outright as well — Apple does it to give one an availability the
// property does not have. The two are one method said twice.
@property (class, readonly) NSInteger version;
+ (NSInteger)version;

// An attribute may lead a block-pointer declarator, which is how every
// enumerate…UsingBlock: in Foundation is written.
- (void)each:(void (__attribute__((noescape)) ^)(NSUInteger idx))block;
- (NSInteger)sortUsing:(NSComparisonResult (^)(id a, id b))cmp;
@end
