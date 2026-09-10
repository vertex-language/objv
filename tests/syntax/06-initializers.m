// §5.10 Initializers

int scalar = 1;
int arithmetic = 1 + 2 * 3;

int array[3] = { 1, 2, 3 };
int trailing[3] = { 1, 2, 3, };
int partial[5] = { 1 };
int nested[2][2] = { { 1, 2 }, { 3, 4 } };
char string[] = "initialized";

struct Point { int x, y; };
struct Point origin = { 0, 0 };

// Designation: both Designator forms, and a chain of them
int designated[5] = { [0] = 1, [4] = 5 };
struct Point named = { .x = 1, .y = 2 };

struct Line { struct Point a, b; };
struct Line chained = { .a.x = 1, .a.y = 2, .b = { 3, 4 } };
int mixedDesignation[4] = { 1, [2] = 3, 4 };

// A compound literal is an initializer list in expression position
struct Point *ptr = &(struct Point){ 1, 2 };
int *arr = (int []){ 1, 2, 3 };
