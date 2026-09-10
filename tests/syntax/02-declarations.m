// §5.1 Storage Class Specifiers, §5.2 Function and Alignment Specifiers,
// §5.3 Type Specifiers

typedef int Integer;
extern int external;
static int internal;
_Thread_local int tls;
__thread int gnuTls;            // the extension spelling of _Thread_local
void storageInBlockScope(void) {
    auto int automatic;
    register int reg;
    (void)automatic; (void)reg;
}

inline int inlineFn(void) { return 0; }
_Noreturn void diverge(void);

_Alignas(16) char aligned16[64];
_Alignas(double) char alignedAsDouble[8];

// TypeSpecifier: the builtin set
void *pv;
char ch;
short sh;
int i;
long l;
long long ll;
unsigned long ul;
signed char sc;
float f;
double db;
long double ld;
_Bool b;
_Complex double cd;

// AtomicTypeSpecifier vs the _Atomic qualifier
_Atomic(int) atomicInt;
_Atomic int alsoAtomic;

// TypeofSpecifier, in all three spellings
int origin;
typeof(origin) t1;
__typeof(origin) t2;
__typeof__(origin) t3;
typeof(int *) t4;

// __auto_type: the type is the initializer's
__auto_type inferred = 42;

// TypedefName
Integer viaTypedef;

// __block, which §5.1 makes a storage class
void usesBlockStorage(void) {
    __block int counter = 0;
    counter++;
}
