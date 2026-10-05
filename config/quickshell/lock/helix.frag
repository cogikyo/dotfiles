#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float phase;
    float time;
    float aspect;
    float busy;
    float busyStart;
    float failStart;
    vec4 pale;
    vec4 strand;
    vec4 deep;
    vec4 alarm;
    vec4 adenine;
    vec4 thymine;
    vec4 guanine;
    vec4 cytosine;
    vec4 s0;
    vec4 s1;
    vec4 s2;
    vec4 s3;
    vec4 s4;
    vec4 s5;
    vec4 s6;
    vec4 s7;
};

const float TAU = 6.28318530718;
const float AMPLITUDE = 0.6;
const float TURN = 4.2;
const float TWIST = TAU / TURN;
const float GROOVE = 2.4;
const float RUNG = TURN / 10.5;
const float LINK = RUNG * 0.5;
const float AA = 0.006;

struct Glow {
    float heat;
    float age;
    float echo;
    vec3 tint;
    vec3 base;
};

float hash(float n) {
    return fract(sin(n * 12.9898 + 78.233) * 43758.5453);
}

float envelope(float age, float attack, float rate, float gain) {
    if (age < 0.0)
        return 0.0;
    return min(1.0, gain * (1.0 - exp(-age * attack)) * exp(-age * rate));
}

float front(float age) {
    return clamp(age * 7.0, 0.0, 1.3);
}

vec3 nucleotide(int base) {
    if (base == 0)
        return adenine.rgb;
    if (base == 1)
        return thymine.rgb;
    if (base == 2)
        return guanine.rgb;
    return cytosine.rgb;
}

float pentagon(vec2 p, float r) {
    const vec3 k = vec3(0.809016994, 0.587785252, 0.726542528);
    p.x = abs(p.x);
    p -= 2.0 * min(dot(vec2(-k.x, k.y), p), 0.0) * vec2(-k.x, k.y);
    p -= 2.0 * min(dot(vec2(k.x, k.y), p), 0.0) * vec2(k.x, k.y);
    p -= vec2(clamp(p.x, -r * k.z, r * k.z), r);
    return length(p) * sign(p.y);
}

void spark(vec4 s, float index, inout Glow a, inout Glow b) {
    if (abs(s.x - index) > 0.5)
        return;
    float age = time - s.y;
    float heat = envelope(age, 14.0, 2.2, 1.58);
    if (s.z < 0.5) {
        if (heat > a.heat) {
            a.heat = heat;
            a.age = age;
            a.tint = s.w > 0.5 ? pale.rgb : a.base;
        }
    } else if (heat > b.heat) {
        b.heat = heat;
        b.age = age;
        b.tint = s.w > 0.5 ? pale.rgb : b.base;
    }
}

void flood(inout Glow g, float heat, vec3 tint) {
    if (heat <= g.heat)
        return;
    g.heat = heat;
    g.age = 10.0;
    g.tint = tint;
    g.echo = 0.0;
}

void backbone(vec2 p, float offset, vec3 hue, inout vec3 color, inout float alpha) {
    float center = floor(p.x / LINK + 0.5);
    for (int n = -2; n <= 2; n++) {
        float j = center + float(n);
        float x = j * LINK;
        float angle = phase + x * TWIST + offset;
        float depth = 0.5 + 0.5 * cos(angle);
        vec2 d = p - vec2(x, AMPLITUDE * sin(angle));
        bool sugar = mod(j, 2.0) < 0.5;
        float radius = mix(0.04, 0.066, depth) * (sugar ? 1.2 : 0.75);
        float c = cos(angle);
        float s = sin(angle);
        float sd = sugar ? pentagon(mat2(c, -s, s, c) * d, radius * 0.82) : length(d) - radius;
        float soft = mix(0.03, 0.007, depth);
        float core = 1.0 - smoothstep(-soft, soft, sd);
        float outside = max(sd, 0.0);
        float glow = exp(-outside * outside / (radius * radius * 2.5)) * 0.28;
        float strength = (core + glow) * mix(0.28, 1.0, depth);
        color += mix(deep.rgb, hue, depth) * strength;
        alpha += strength;
    }
}

void base(vec2 p, float x, float root, float tip, float bulb, float depth, Glow g, inout vec3 color, inout float alpha) {
    float span = tip - root;
    if (abs(span) < 0.004)
        return;
    float t = clamp((p.y - root) / span, 0.0, 1.0);
    float r = mix(0.007, bulb, smoothstep(0.3, 1.0, t)) * mix(0.7, 1.15, depth);
    float d = length(vec2(p.x - x, p.y - root - span * t)) - r;
    float core = 1.0 - smoothstep(-AA, AA, d);

    float f = front(g.age);
    float flow = (1.0 - smoothstep(f - 0.3, f, t)) * g.heat;
    float heat = max(flow, g.echo);
    vec3 hue = flow >= g.echo ? g.tint : g.base;

    float lit = mix(0.4, 1.0, depth);
    float sigma = 0.012 + 0.04 * heat;
    float od = max(d, 0.0);
    float bloom = exp(-od * od / (sigma * sigma)) * heat * 0.85;

    vec3 rest = mix(deep.rgb, g.base, 0.35);
    vec3 hot = hue * (1.0 + 0.45 * heat) + vec3(0.28) * heat * heat;
    vec3 tone = mix(rest, hot, smoothstep(0.0, 1.0, heat));
    float strength = core * (0.45 + 0.55 * heat) * lit;
    color += tone * strength + hue * bloom;
    alpha += strength + bloom;
}

void main() {
    vec2 p = (qt_TexCoord0 - 0.5) * vec2(aspect, 1.0) * 2.0;
    vec3 color = vec3(0.0);
    float alpha = 0.0;

    backbone(p, 0.0, pale.rgb, color, alpha);
    backbone(p, GROOVE, strand.rgb, color, alpha);

    float scan = fract((time - busyStart) / 1.4) * 1.4 - 0.2;
    float failing = envelope(time - failStart, 30.0, 2.4, 1.1);
    float center = floor(p.x / RUNG + 0.5);

    for (int n = -1; n <= 1; n++) {
        float index = center + float(n);
        float x = index * RUNG;
        float angle = phase + x * TWIST;
        float ya = AMPLITUDE * sin(angle);
        float yb = AMPLITUDE * sin(angle + GROOVE);
        float depth = 0.5 + 0.5 * cos(angle + GROOVE * 0.5);
        float dx = p.x - x;

        int pair = int(floor(hash(index) * 4.0));
        int baseA = pair;
        int baseB = pair ^ 1;
        bool purine = baseA == 0 || baseA == 2;
        float bonds = baseA >= 2 ? 3.0 : 2.0;

        float dir = sign(yb - ya);
        float meet = ya + (yb - ya) * (purine ? 0.56 : 0.44);
        float gap = min(0.05, abs(yb - ya) * 0.12);
        float endA = meet - dir * gap;
        float startB = meet + dir * gap;

        Glow a = Glow(0.0, 10.0, 0.0, nucleotide(baseA), nucleotide(baseA));
        Glow b = Glow(0.0, 10.0, 0.0, nucleotide(baseB), nucleotide(baseB));
        spark(s0, index, a, b);
        spark(s1, index, a, b);
        spark(s2, index, a, b);
        spark(s3, index, a, b);
        spark(s4, index, a, b);
        spark(s5, index, a, b);
        spark(s6, index, a, b);
        spark(s7, index, a, b);

        float bondA = a.heat * smoothstep(0.85, 1.15, front(a.age));
        float bondB = b.heat * smoothstep(0.85, 1.15, front(b.age));
        a.echo = b.tint == pale.rgb ? 0.0 : 0.35 * bondB;
        b.echo = a.tint == pale.rgb ? 0.0 : 0.35 * bondA;

        float u = 0.5 + 0.5 * x / aspect;
        float sweep = busy * exp(-pow(u - scan, 2.0) / 0.004);
        flood(a, sweep, pale.rgb);
        flood(b, sweep, pale.rgb);
        flood(a, failing, alarm.rgb);
        flood(b, failing, alarm.rgb);
        bondA = max(bondA, max(sweep, failing));

        base(p, x, ya, endA, purine ? 0.026 : 0.021, depth, a, color, alpha);
        base(p, x, yb, startB, purine ? 0.021 : 0.026, depth, b, color, alpha);

        float lo = min(endA, startB);
        float hi = max(endA, startB);
        if (hi - lo < 0.002)
            continue;
        float inGap = smoothstep(lo - AA, lo + AA, p.y) * (1.0 - smoothstep(hi - AA, hi + AA, p.y));
        if (inGap <= 0.0)
            continue;
        float spacing = 0.024;
        float ly = (fract((p.y - meet) / spacing + 0.5) - 0.5) * spacing;
        float spread = 0.02;
        float first = -0.5 * (bonds - 1.0) * spread;
        float dots = 0.0;
        for (int k = 0; k < 3; k++) {
            if (float(k) >= bonds)
                break;
            float o = first + float(k) * spread;
            dots += 1.0 - smoothstep(-AA, AA, length(vec2(dx - o, ly)) - 0.006);
        }
        float glow = max(bondA, bondB);
        float w = clamp((p.y - endA) / (startB - endA), 0.0, 1.0);
        vec3 hue = mix(pale.rgb, mix(a.tint, b.tint, w), glow);
        float strength = dots * inGap * mix(0.4, 1.0, depth) * (0.3 + 0.7 * glow);
        color += hue * strength;
        alpha += strength;
    }

    float u = qt_TexCoord0.x;
    float fade = smoothstep(0.0, 0.22, u) * (1.0 - smoothstep(0.78, 1.0, u));
    fragColor = vec4(color, clamp(alpha, 0.0, 1.0)) * fade * qt_Opacity;
}
