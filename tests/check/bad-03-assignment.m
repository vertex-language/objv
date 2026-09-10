// Assignments between object pointers.

@protocol Marker
- (void)mark;
@end

@interface Animal : NSObject
@end

@interface Dog : Animal
@end

@interface Plant : NSObject
@end

@implementation Dog
- (void)assignments {
    Dog *dog;
    Animal *animal = dog;           // a subclass is its superclass
    Dog *back = animal;             // expect: the classes are unrelated
    Plant *plant = dog;             // expect: the classes are unrelated
    id<Marker> marker = dog;        // expect: not known to conform to 'Marker'

    NSArray<NSString *> *strings;
    NSArray<NSNumber *> *numbers = strings;   // expect: the type arguments differ

    int n = dog;                    // expect: one is a pointer and the other an integer
    (void)animal; (void)back; (void)plant; (void)marker; (void)numbers; (void)n;
}
@end
