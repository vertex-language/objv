// Arguments past the eighth register go on the stack, and Apple's arm64
// packs them at their own size: CGImageCreate's shouldInterpolate is one
// byte at sp+8 and its rendering intent four at sp+12, where the base
// AAPCS64 would give each an eightbyte. CoreGraphics is compiled by Apple,
// so it is the other side of the call, and the image reports back what it
// was given. Getting this wrong handed CoreGraphics a garbage intent, which
// is what crashed cwindow_present inside CATransaction commit.
// frameworks: CoreGraphics
#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <CoreGraphics/CoreGraphics.h>

static void freePixels(void *info, const void *data, size_t size) { free((void *)data); }

static void make(bool interpolate, CGColorRenderingIntent intent) {
    size_t w = 8, h = 4, size = w * h * 4;
    CGDataProviderRef provider = CGDataProviderCreateWithData(NULL, calloc(1, size), size, freePixels);
    CGColorSpaceRef space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
    const CGFloat decode[] = { 0, 1, 0, 1, 0, 1 };
    CGImageRef image = CGImageCreate(w, h, 8, 32, w * 4, space, (CGBitmapInfo)kCGImageAlphaNoneSkipLast,
                                     provider, intent == kCGRenderingIntentSaturation ? decode : NULL,
                                     interpolate, intent);
    printf("interp %d intent %d decode %d\n", CGImageGetShouldInterpolate(image),
           CGImageGetRenderingIntent(image), CGImageGetDecode(image) != NULL);
    CGImageRelease(image);
    CGColorSpaceRelease(space);
    CGDataProviderRelease(provider);
}

// The same packing between two functions this compiler builds: chars,
// shorts and bools past the registers, interleaved with ints and doubles.
__attribute__((noinline)) static long packed(long r0, long r1, long r2, long r3, long r4, long r5, long r6,
                                            long r7, char a, bool b, short c, int d, char e, double f,
                                            unsigned char g, short h) {
    return r0 + r7 + a * 1 + b * 10 + c * 100 + d * 1000 + e * 3 + (long)f * 7 + g * 11 + h * 13;
}

int main(void) {
    make(false, kCGRenderingIntentDefault);
    make(true, kCGRenderingIntentPerceptual);
    make(false, kCGRenderingIntentRelativeColorimetric);
    make(true, kCGRenderingIntentSaturation);
    printf("%ld\n", packed(1, 0, 0, 0, 0, 0, 0, 2, -3, true, -400, 5, 6, 7.5, 250, -9));
    return 0;
}
