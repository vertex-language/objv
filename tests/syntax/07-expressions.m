// §6.1 Primary Expressions, §6.2 Postfix, §6.5 Unary and Cast,
// §6.6 Binary, §6.7 Conditional and Assignment

struct Point { int x, y; };
int f(int);
int g(void);
int arr[10];
struct Point p;
struct Point *pp;

void expressions(void) {
    int i = 0;
    int *ptr = &i;

    // PostfixExpression
    i = arr[0];
    i = f(1);
    i = g();
    i = p.x;
    i = pp->x;
    i++; i--;
    ++i; --i;

    // UnaryOperator
    ptr = &i;
    i = *ptr;
    i = +i;
    i = -i;
    i = ~i;
    i = !i;

    // sizeof and the two _Alignof spellings
    i = sizeof i;
    i = sizeof(int);
    i = sizeof(struct Point);
    i = _Alignof(int);
    i = __alignof(int);
    i = __alignof__ i;

    // CastExpression
    i = (int)1.5;
    ptr = (int *)0;
    i = (int)(char)i;

    // The ten binary levels of §6.6
    i = 1 * 2 / 3 % 4;
    i = 1 + 2 - 3;
    i = 1 << 2 >> 3;
    i = 1 < 2;  i = 1 > 2;  i = 1 <= 2;  i = 1 >= 2;
    i = 1 == 2; i = 1 != 2;
    i = 1 & 2;  i = 1 ^ 2;  i = 1 | 2;
    i = 1 && 2; i = 1 || 2;

    // Precedence, associativity, and the comma operator
    i = 1 + 2 * 3 == 7 && !0;
    i = (1, 2, 3);

    // ConditionalExpression, and the assignment operators
    i = i ? 1 : 2;
    i = i ? i ? 1 : 2 : 3;
    i = 1; i += 1; i -= 1; i *= 1; i /= 1; i %= 1;
    i <<= 1; i >>= 1; i &= 1; i ^= 1; i |= 1;

    // GenericSelection
    i = _Generic(i, int: 1, float: 2, default: 0);

    // StatementExpression, the extension every non-trivial macro rests on
    i = ({ int tmp = f(i); tmp * 2; });
}
