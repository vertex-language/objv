// §5.7 Declarators

int plain;
int *pointer;
int **pointerToPointer;
int *const constPointer;
const int *pointerToConst;
int *restrict restricted;
int * volatile volatilePointer;

int array[10];
int array2D[3][4];
int incomplete[];
// The qualified, static and [*] array forms, which §5.7 admits and which are
// legal only in a parameter list
void qualifiedArrays(int a[const 10], int b[static 10],
                     int c[const static 10], int d[*]);

int function(void);
int noPrototype();
int params(int a, int b);
int unnamedParams(int, int);
int variadic(const char *fmt, ...);
int krStyle(a, b) int a; int b; { return a + b; }

int (*functionPointer)(int, int);
int *(*returnsPointer)(void);
int (*arrayOfPointers[3])(void);
int (*(*complicated)(int))[4];

// Pointer's second alternative: the block pointer of §5.7
void (^simpleBlock)(void);
int (^blockReturningInt)(int, int);
void (^__strong ownedBlock)(void);
void (^(^blockReturningBlock)(void))(void);

// AbstractDeclarator, in the positions that take one
int sized = sizeof(int *);
int sizedArray = sizeof(int [10]);
int sizedFn = sizeof(int (*)(void));
int sizedBlock = sizeof(void (^)(id));
