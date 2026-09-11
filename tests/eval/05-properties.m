// Dot syntax and object subscripting: the two places where an expression
// that looks like C is a message.
//
// Neither is an lvalue. `obj.name = x` is a call to setName:, which is why
// a compound assignment through one is a getter, an operator, and a setter,
// with the receiver evaluated exactly once.

@interface Box : NSObject
@property (nonatomic) int value;
@property (nonatomic, copy) NSString *label;
@property (nonatomic, readonly) int doubled;
@end

@implementation Box
- (int)value { return 0; }
- (void)setValue:(int)v { }
- (NSString *)label { return 0; }
- (void)setLabel:(NSString *)s { }
- (int)doubled { return self.value * 2; }
@end

int reads(Box *b) { return b.value; }
// vir: callind

void writes(Box *b, NSString *s) {
    b.value = 3;
    b.label = s;
}

void compound(Box *b) {
    b.value += 1;
    b.value++;
    --b.value;
}

id indexed(NSArray *a, NSMutableArray *m, id x) {
    m[0] = x;
    return a[1];
}
// vir: "objectAtIndexedSubscript:"
// vir: "setObject:atIndexedSubscript:"

id keyed(NSDictionary *d, NSMutableDictionary *m, id k, id v) {
    m[k] = v;
    return d[k];
}
// vir: "objectForKeyedSubscript:"
// vir: "setObject:forKeyedSubscript:"

// Dot syntax may name the *getter* rather than the property, which is what a
// getter= attribute leaves behind: the property is `dirty` and the name a
// reader writes is `isDirty`. Nothing declares a property with that name, so
// the send is resolved from the method.
@interface Flagged : NSObject
@property (nonatomic, assign, getter=isDirty) BOOL dirty;
- (NSString *)describe;
@end
@implementation Flagged
- (NSString *)describe { return self.isDirty ? @"dirty" : @"clean"; }
@end
// vir: @_OBJC_SELECTOR_REFERENCES_
