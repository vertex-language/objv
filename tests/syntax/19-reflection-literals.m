// §6.4 Reflection Expressions, §6.8 Object Literals

@class NSString;
@protocol NSCopying;

@interface Reflect
- (void)setObject:(id)o forKey:(id)k;
- (void)b:(int)x :(int)y;
@end

typedef struct { int x, y; } CGPoint;

@implementation Reflect
- (void)setObject:(id)o forKey:(id)k { (void)o; (void)k; }
- (void)b:(int)x :(int)y { (void)x; (void)y; }

- (void)reflection {
    // SelectorExpression: a unary selector, a keyword selector, and one
    // whose pieces are nameless
    SEL unary = @selector(description);
    SEL keyword = @selector(setObject:forKey:);
    SEL nameless = @selector(a::);
    SEL keywordSpelled = @selector(default:);

    // ProtocolExpression
    Protocol *proto = @protocol(NSCopying);

    // EncodeExpression
    const char *e1 = @encode(int);
    const char *e2 = @encode(id);
    const char *e3 = @encode(CGPoint);
    const char *e4 = @encode(void (^)(id));

    (void)unary; (void)keyword; (void)nameless; (void)keywordSpelled;
    (void)proto; (void)e1; (void)e2; (void)e3; (void)e4;
}

- (void)literals {
    int count = 3;

    // BoxedExpression, in every form §6.8 admits
    id boxedInt = @42;
    id boxedNegative = @-1;
    id boxedFloat = @3.5f;
    id boxedChar = @'c';
    id boxedYes = @__objc_yes;
    id boxedNo = @__objc_no;
    id boxedExpr = @(count + 1);
    id boxedCall = @([self hash]);

    // A string object, and the sequences of §6.1
    id string = @"literal";
    id joined = @"one" @"two";

    // ArrayLiteral, with and without a trailing comma
    id empty = @[];
    id array = @[@1, @2, @3];
    id trailing = @[@1, @2,];
    id nested = @[@[@1], @[@2]];

    // DictionaryLiteral
    id emptyDict = @{};
    id dict = @{@"a": @1, @"b": @2};
    id trailingDict = @{@"a": @1,};
    id nestedDict = @{@"outer": @{@"inner": @[@1]}};

    (void)boxedInt; (void)boxedNegative; (void)boxedFloat; (void)boxedChar;
    (void)boxedYes; (void)boxedNo; (void)boxedExpr; (void)boxedCall;
    (void)string; (void)joined; (void)empty; (void)array; (void)trailing;
    (void)nested; (void)emptyDict; (void)dict; (void)trailingDict;
    (void)nestedDict;
}
@end
