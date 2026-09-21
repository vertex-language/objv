// String literals: length, indexing, escapes, adjacent concatenation.
#include <stdio.h>
#include <string.h>

int main(void) {
    const char *s = "hello, " "world";
    printf("%s %zu %c\n", s, strlen(s), s[7]);
    printf("%zu\n", sizeof "abc");
    printf("[%s]\n", "tab\there \"quoted\" \\ \x41\101");
    printf("%d\n", "xyz"[1]);
    return 0;
}
