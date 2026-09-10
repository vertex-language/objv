// §6.10 Availability Checks, §7.4 Assembly

@interface Guarded
- (void)modern;
@end

@implementation Guarded
- (void)modern { }

- (void)checks {
    // Both spellings of the same construct
    if (@available(macOS 10.12, *)) {
        [self modern];
    }
    if (__builtin_available(iOS 13.0, *)) {
        [self modern];
    }

    // Several platforms, and every VersionTuple shape
    if (@available(macOS 10.12.4, iOS 13, tvOS 12.1, *)) {
        [self modern];
    }

    // As the operand of a !, which §6.10 also lets guard a declaration
    if (!@available(macOS 11.0, *)) {
        return;
    }

    // An ordinary primary expression: it may appear wherever one may
    int supported = @available(macOS 10.15, *) ? 1 : 0;
    (void)supported;
}
@end

// AsmStatement as an ExternalDeclaration: file-scope assembly, no operands
asm(".globl _objv_marker");
__asm__(".p2align 2");

void inlineAssembly(int x) {
    // The interior is a BalancedTokenSequence, whose shape is the target's
    asm("nop");
    __asm__ volatile("nop");
    __asm__ volatile("mov %0, %0" : "=r"(x) : "r"(x) : "memory");
    asm goto("b %l0" : : : : done);
done:
    return;
}
