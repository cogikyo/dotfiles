#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec2 inner;
};

layout(binding = 1) uniform sampler2D source;

void main() {
    vec2 p = vec2(1.0 - qt_TexCoord0.x, qt_TexCoord0.y);
    float r = length(p);
    float core = r / max(length(p / inner), 1e-6);
    float t = clamp((r - core) / (1.0 - core), 0.0, 1.0);
    fragColor = texture(source, qt_TexCoord0) * (1.0 - smoothstep(0.0, 1.0, t)) * qt_Opacity;
}
