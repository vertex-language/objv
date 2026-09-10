// §7 Statements, §7.1 Iteration

int f(int);

void statements(int n) {
    // LabeledStatement, and the label namespace: `done` may also be a type
    if (n) goto done;

    // CompoundStatement, with declarations and statements interleaved
    {
        int local = 1;
        local++;
        int later = local;
        (void)later;
    }

    // ExpressionStatement and the null statement
    f(n);
    ;

    // SelectionStatement, including the dangling else
    if (n) f(1);
    if (n) f(1); else f(2);
    if (n) if (n) f(1); else f(2);

    switch (n) {
    case 1:
        f(1);
        break;
    case 2:
    case 3:
        f(2);
        break;
    default:
        break;
    }

    // IterationStatement: all four of §7.1's C forms
    while (n) { n--; }
    do { n--; } while (n);
    for (int i = 0; i < n; i++) { f(i); }
    for (n = 0; n < 10; n++) ;
    for (;;) { break; }
    for (int i = 0, j = 1; i < j; i++, j--) ;

    // JumpStatement
    for (int i = 0; i < n; i++) {
        if (i == 1) continue;
        if (i == 2) break;
    }
done:
    return;
}

int returnsValue(void) { return 1; }
void returnsNothing(void) { return; }
