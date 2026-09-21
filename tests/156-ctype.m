// Character arithmetic and <ctype.h>: classifying and converting a string.
#include <stdio.h>
#include <ctype.h>

int main(void) {
    const char *s = "Hello, World 2026!";
    int up = 0, lo = 0, dig = 0, sp = 0, pun = 0;
    for (const char *p = s; *p; p++) {
        unsigned char c = (unsigned char)*p;
        up += isupper(c) != 0; lo += islower(c) != 0; dig += isdigit(c) != 0;
        sp += isspace(c) != 0; pun += ispunct(c) != 0;
    }
    printf("%d %d %d %d %d\n", up, lo, dig, sp, pun);
    char rot[32];
    int i = 0;
    for (const char *p = s; *p; p++, i++) {
        char c = *p;
        if (c >= 'a' && c <= 'z') c = (char)('a' + (c - 'a' + 13) % 26);
        else if (c >= 'A' && c <= 'Z') c = (char)('A' + (c - 'A' + 13) % 26);
        rot[i] = c;
    }
    rot[i] = 0;
    printf("%s %c %d\n", rot, toupper('q'), '7' - '0');
    return 0;
}
