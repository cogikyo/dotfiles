#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec2 area;
    vec4 edges;
};

layout(binding = 1) uniform sampler2D source;

float ramp(float d, float e) {
    return e > 0.0 ? smoothstep(0.0, e, d) : 1.0;
}

void main() {
    vec2 p = qt_TexCoord0 * area;
    float a = ramp(p.x, edges.x) * ramp(p.y, edges.y) * ramp(area.x - p.x, edges.z) * ramp(area.y - p.y, edges.w);
    fragColor = texture(source, qt_TexCoord0) * a * qt_Opacity;
}
