// A message with more arguments than registers, ints and doubles mixed.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Wide : NSObject
- (double)a:(int)a b:(double)b c:(long)c d:(double)d e:(int)e f:(double)f
          g:(int)g h:(double)h i:(int)i j:(double)j k:(char)k l:(float)l
          m:(short)m n:(double)n;
@end
@implementation Wide
- (double)a:(int)a b:(double)b c:(long)c d:(double)d e:(int)e f:(double)f
          g:(int)g h:(double)h i:(int)i j:(double)j k:(char)k l:(float)l
          m:(short)m n:(double)n {
    return a + b + c + d + e + f + g + h + i + j + k * 100 + l * 1000 + m * 10000 + n * 100000;
}
@end

int main(void) {
    double r = [[Wide new] a:1 b:2 c:3 d:4 e:5 f:6 g:7 h:8 i:9 j:10 k:11 l:0.5f m:13 n:0.25];
    printf("%.2f\n", r);
    return 0;
}
