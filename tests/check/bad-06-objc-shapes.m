// The Objective-C constructs whose operands are constrained.

@interface Shapes : NSObject {
@private
    int _hidden;
}
- (void)ping;
@end

@interface Other : NSObject
@end

@implementation Shapes
- (void)ping { }

- (void)mistakes {
    NSObject value;                  // expect: cannot be declared by value
    unsigned long n = sizeof(NSObject);  // expect: invalid application of 'sizeof'

    for (int i in @[@1]) { }         // expect: loop variable of a fast enumeration
    for (id x in @"not a collection") { }

    @synchronized (7) { }            // expect: @synchronized takes an object
    @throw 7;                        // expect: @throw takes an object
    @throw;                          // expect: valid only inside a @catch

    id a = @[7];                     // expect: which is not an object
    id b = @{@"k": 7};               // expect: which is not an object

    void (^blk)(void) = ^{ };
    (*blk)();                        // expect: block pointer cannot be dereferenced
    // A block is an object -- its first word is an isa -- so a send to one
    // is the send to id, which the runtime resolves and this does not
    // report. Sending to something that is not an object still is.
    [blk ping];
    [n ping];                        // expect: which is not an object pointer

    Other *other;
    other->_hidden = 1;              // expect: has no instance variable named '_hidden'
    (void)n; (void)a; (void)b;
}
@end
