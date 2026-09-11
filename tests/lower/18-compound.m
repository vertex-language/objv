// §6.5.2.5's compound literal.
//
// `(struct Vec){ 1, 2 }` is an unnamed *object* and not a value: it has a
// lifetime, it has an address, and it is an lvalue. Which lifetime is the
// scope it was written in, and that is the whole of the difference between
// the two halves — automatic inside a function, static at file scope.

struct Vec { double x, y; };

// At file scope the object is a global with a constant initializer, and what
// an initializer can do with it is take its address.
struct Vec *origin = &(struct Vec){ 0, 0 };
// vir: internal global rw @_compound
// vir: export global rw @_origin ptr align 8 = @_compound

const int *primes = (const int[]){ 2, 3, 5 };
// vir: = { 2, 3, 5 }

// Inside a function it is a frame slot, re-initialized every time control
// reaches the literal. The storage is the same each time, which is what "the
// enclosing block" means.
double sum(struct Vec a, struct Vec b) {
    struct Vec v = (struct Vec){ a.x + b.x, a.y + b.y };
    return v.x + v.y;
}
// vir: %compound = ptr.alloc 16

// It is an lvalue, which is what makes these mean something.
int pick(int i) { return (int[]){ 7, 8, 9 }[i]; }
double first(void) { return (&(struct Vec){ 1, 2 })->x; }
// vir: ptr.add
