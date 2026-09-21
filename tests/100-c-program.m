// Plain C put together: a word-frequency table with an open-addressing hash,
// sorted output.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

struct entry { char word[16]; int count; };
static struct entry table[64];

static unsigned hash(const char *s) {
    unsigned h = 2166136261u;
    while (*s) h = (h ^ (unsigned char)*s++) * 16777619u;
    return h;
}

static void add(const char *w) {
    for (unsigned i = hash(w) % 64;; i = (i + 1) % 64) {
        if (!table[i].word[0]) { strcpy(table[i].word, w); table[i].count = 1; return; }
        if (!strcmp(table[i].word, w)) { table[i].count++; return; }
    }
}

static int cmp(const void *a, const void *b) {
    const struct entry *x = a, *y = b;
    if (x->count != y->count) return y->count - x->count;
    return strcmp(x->word, y->word);
}

int main(void) {
    const char *text = "the cat and the dog and the bird saw a cat";
    char w[16];
    int n = 0;
    for (const char *p = text;; p++) {
        if (isalpha((unsigned char)*p)) { w[n++] = (char)tolower(*p); continue; }
        if (n) { w[n] = 0; add(w); n = 0; }
        if (!*p) break;
    }
    qsort(table, 64, sizeof table[0], cmp);
    for (int i = 0; i < 64 && table[i].count; i++) printf("%s %d\n", table[i].word, table[i].count);
    return 0;
}
