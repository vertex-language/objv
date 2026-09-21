// The method called is chosen by the receiver's class at run time, through
// id and through a superclass-typed pointer.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Shape : NSObject
- (double)area;
@end
@implementation Shape
- (double)area { return 0; }
@end

@interface Square : Shape
@property (nonatomic) double side;
@end
@implementation Square
- (double)area { return _side * _side; }
@end

@interface Circle : Shape
@property (nonatomic) double r;
@end
@implementation Circle
- (double)area { return 3 * _r * _r; }
@end

int main(void) {
    Square *s = [Square new]; s.side = 3;
    Circle *c = [Circle new]; c.r = 2;
    Shape *shapes[] = { s, c, [Shape new] };
    for (int i = 0; i < 3; i++) printf("%g\n", [shapes[i] area]);
    id any = c;
    printf("%g\n", [any area]);
    return 0;
}
