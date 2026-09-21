// A char array edited in place with libc's string functions.
#include <stdio.h>
#include <string.h>

void reverse(char *s) {
    for (size_t i = 0, j = strlen(s); i + 1 < j; i++, j--) {
        char t = s[i]; s[i] = s[j - 1]; s[j - 1] = t;
    }
}

int main(void) {
    char buf[32] = "objective";
    strcat(buf, "-c");
    printf("%s %zu\n", buf, strlen(buf));
    reverse(buf);
    printf("%s\n", buf);
    printf("%d %d\n", strcmp("abc", "abd") < 0, memcmp("aa", "aa", 2));
    char copy[32];
    memcpy(copy, buf, strlen(buf) + 1);
    printf("%s %s\n", copy, strchr(copy, 'j'));
    return 0;
}
