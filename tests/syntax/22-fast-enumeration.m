// §7.1's last two IterationStatement alternatives

@class NSString, NSArray, NSDictionary;

@interface Enumerable
@property (nonatomic, copy) NSArray<NSString *> *items;
@end

@implementation Enumerable
- (void)enumerate {
    // The declaration form
    for (NSString *s in self.items) {
        (void)s;
    }

    // The expression form: the variable already exists
    id element;
    for (element in self.items) {
        (void)element;
    }

    // Qualified and generic loop variables
    for (id<NSObject> o in self.items) { (void)o; }
    for (__strong NSString *s in self.items) { (void)s; }
    for (__kindof NSString *s in self.items) { (void)s; }

    // The collection is any expression
    for (NSString *s in [self items]) { (void)s; }
    for (NSString *s in @[@"a", @"b"]) { (void)s; }

    // They nest, and mix with C loops
    for (NSString *outer in self.items) {
        for (int i = 0; i < 3; i++) {
            for (NSString *inner in self.items) {
                if (i) continue;
                (void)outer; (void)inner;
            }
        }
    }

    // `in` is contextual: it is an ordinary identifier everywhere else
    int in = 1;
    for (int i = in; i < 2; i++) { (void)i; }
}
@end
