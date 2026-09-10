// §5.8 Structures, Unions, and Enumerations

struct Point { int x; int y; };
struct Empty { };
struct Tagged;
union Value { int i; float f; void *p; };

struct WithBitfields {
    unsigned int flag : 1;
    unsigned int count : 15;
    unsigned int : 0;           // an unnamed bit-field
    int plain;
};

struct Nested {
    struct Inner { int a; } inner;
    union { int x; float y; } anon;
};

struct MultipleDeclarators { int a, *b, c[4]; };

struct WithAssert {
    int size;
    _Static_assert(sizeof(int) >= 4, "int is too small");
};

enum Plain { PlainA, PlainB, PlainC };
enum WithValues { ValueA = 1, ValueB = 2, ValueC = ValueA + ValueB };
enum TrailingComma { TrailA, TrailB, };
enum Forward;

// The fixed underlying type of §5.8, which is what NS_ENUM expands to
typedef long NSInteger;
enum Fixed : NSInteger { FixedA, FixedB };
enum FixedForward : NSInteger;

// NS_ENUM(NSInteger, State) expands to exactly this
enum State : NSInteger State; enum State : NSInteger { Idle, Busy };

// NS_OPTIONS, which is the same shape
typedef unsigned long NSUInteger;
enum Mask : NSUInteger Mask; enum Mask : NSUInteger {
    MaskNone  = 0,
    MaskFirst = 1 << 0,
    MaskAll   = MaskNone | MaskFirst,
};
