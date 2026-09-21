// Homogeneous floating-point aggregates travel in float registers.
#include <stdio.h>

struct vec2 { float x, y; };
struct vec4d { double a, b, c, d; };
struct vec3 { float x, y, z; };

struct vec2 add2(struct vec2 p, struct vec2 q) { return (struct vec2){ p.x + q.x, p.y + q.y }; }
struct vec4d scale(struct vec4d v, double k) { return (struct vec4d){ v.a * k, v.b * k, v.c * k, v.d * k }; }
float len2(struct vec3 v) { return v.x * v.x + v.y * v.y + v.z * v.z; }

int main(void) {
    struct vec2 r = add2((struct vec2){ 1.5f, 2 }, (struct vec2){ 0.25f, -4 });
    struct vec4d s = scale((struct vec4d){ 1, 2, 3, 4 }, 0.5);
    printf("%g %g\n", r.x, r.y);
    printf("%g %g %g %g\n", s.a, s.b, s.c, s.d);
    printf("%g\n", len2((struct vec3){ 1, 2, 3 }));
    return 0;
}
