in vec4 gl_FragCoord;

#request uniform "screen" screen
uniform ivec2 screen;

#request uniform "audio_sz" audio_sz
uniform int audio_sz;

#request uniform "audio_l" audio_l
#request transform audio_l "window"
#request transform audio_l "fft"
#request transform audio_l "gravity"
#request transform audio_l "avg"
uniform sampler1D audio_l;

#request uniform "audio_r" audio_r
#request transform audio_r "window"
#request transform audio_r "fft"
#request transform audio_r "gravity"
#request transform audio_r "avg"
uniform sampler1D audio_r;

out vec4 fragment;
#include ":util/smooth.glsl"

#request setavgframes 7
#request setgravitystep 6
#request setsmoothfactor 0.04

#define BAR_WIDTH 2.0
#define BAR_GAP 2.0
#define AMPLIFY 1.5
#define EDGE_START 0.15
#define EDGE_END 0.25
#define EDGE_POWER 2.2

float falloff(float q) {
    if (q < EDGE_START)
        return pow(q / EDGE_START, EDGE_POWER);
    if (q > 1.0 - EDGE_END)
        return pow((1.0 - q) / EDGE_END, EDGE_POWER);
    return 1.0;
}

void main() {
    fragment = vec4(0.0);
    float section = BAR_WIDTH + BAR_GAP;
    float bars = floor(float(screen.x) / section);
    float i = floor(gl_FragCoord.x / section);
    if (mod(gl_FragCoord.x, section) >= BAR_WIDTH || i >= bars)
        return;

    float x = 2.0 * (i + 0.5) / bars - 1.0;
    float q = 1.0 - abs(x);
    float v = x < 0.0 ? smooth_audio(audio_l, audio_sz, q) : smooth_audio(audio_r, audio_sz, q);
    v *= falloff(q);
    float d = gl_FragCoord.y / float(screen.y);
    if (d < v * AMPLIFY)
        fragment = (STDIN.a > 0.0 ? STDIN : #6380ec) * (3.0 * d + 0.69);
}
