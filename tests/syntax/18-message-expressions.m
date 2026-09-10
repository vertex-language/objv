// §6.3 Message Expressions, §6.2's overloaded . and []

@class NSString, NSArray, NSNumber;

@interface NSObject
+ (instancetype)alloc;
- (instancetype)init;
- (id)copy;
@end

@interface Target : NSObject
@property (nonatomic) int value;
+ (instancetype)shared;
- (void)unary;
- (void)take:(int)x;
- (void)take:(int)x and:(int)y;
// The four methods §6.2's subscripting is rewritten to
- (id)objectAtIndexedSubscript:(int)i;
- (void)setObject:(id)object atIndexedSubscript:(int)i;
- (id)objectForKeyedSubscript:(id)key;
- (void)setObject:(id)object forKeyedSubscript:(id)key;
- (void)b:(int)x :(int)y;
- (void)variadic:(id)first, ...;
@end

@interface Target (Sub)
@end

@implementation Target
- (void)unary { }
- (void)take:(int)x { (void)x; }
- (void)take:(int)x and:(int)y { (void)x; (void)y; }
- (id)objectAtIndexedSubscript:(int)i { (void)i; return nil; }
- (void)setObject:(id)object atIndexedSubscript:(int)i { (void)object; (void)i; }
- (id)objectForKeyedSubscript:(id)key { (void)key; return nil; }
- (void)setObject:(id)object forKeyedSubscript:(id)key { (void)object; (void)key; }
- (void)b:(int)x :(int)y { (void)x; (void)y; }
- (void)variadic:(id)first, ... { (void)first; }
+ (instancetype)shared { return nil; }

- (void)sends {
    // Receiver: an expression, a class name, and super
    [self unary];
    [Target shared];
    [super init];

    // Nested sends, which is how every object is made
    id obj = [[Target alloc] init];
    id copied = [[[Target alloc] init] copy];

    // KeywordArgument, one and many
    [self take:1];
    [self take:1 and:2];

    // A nameless keyword piece: the selector is b::
    [self b:1 :2];

    // The trailing arguments of a variadic method, on the final keyword
    [self variadic:obj, obj, (id)0];

    // Message sends as arguments to message sends
    [self take:[self value] and:(int)[obj hash]];

    // §6.2's overloaded '.': property dot syntax, reading and writing
    self.value = 1;
    int v = self.value;
    self.value += 1;
    (void)v;

    // §6.2's overloaded '[]': keyed and indexed subscripting
    id first = self[0];
    id keyed = self[@"key"];
    self[0] = obj;
    self[@"key"] = obj;
    (void)first; (void)keyed; (void)copied;
}
@end
