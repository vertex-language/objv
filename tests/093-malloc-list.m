// A heap-allocated linked list, built, reversed and freed.
#include <stdio.h>
#include <stdlib.h>

struct node { int v; struct node *next; };

struct node *push(struct node *head, int v) {
    struct node *n = malloc(sizeof *n);
    n->v = v;
    n->next = head;
    return n;
}

struct node *reverse(struct node *h) {
    struct node *prev = NULL;
    while (h) { struct node *next = h->next; h->next = prev; prev = h; h = next; }
    return prev;
}

int main(void) {
    struct node *h = NULL;
    for (int i = 1; i <= 5; i++) h = push(h, i * i);
    h = reverse(h);
    for (struct node *n = h; n; n = n->next) printf("%d ", n->v);
    printf("\n");
    while (h) { struct node *n = h->next; free(h); h = n; }
    int *grow = calloc(2, sizeof *grow);
    grow = realloc(grow, 100 * sizeof *grow);
    grow[99] = 7;
    printf("%d %d\n", grow[0], grow[99]);
    free(grow);
    return 0;
}
