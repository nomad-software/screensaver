#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 resolution; // Screen size in pixels.
uniform float glowSpread; // Distance between glow samples in screen pixels.
uniform float time;

out vec4 finalColor;

const float curvature = 0.04; // How much the tube bulges.
const float aberration = 1.4; // Colour fringing in glow spread units.
const float jitter = 0.5; // Sideways jitter of the scanlines in glow spread units.

float hash(vec2 p)
{
    return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

// Bend the picture like the curved glass of an old tube. The middle of each
// edge stays put while the corners are pushed off screen.
vec2 warp(vec2 uv)
{
    vec2 c = uv * 2.0 - 1.0;
    c *= (1.0 + curvature * dot(c, c)) / (1.0 + curvature);
    return c * 0.5 + 0.5;
}

// Sample the picture with the red and blue guns misaligned sideways.
vec3 scene(vec2 uv, float shift)
{
    if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) {
        return vec3(0.0);
    }
    return vec3(
        texture(texture0, uv + vec2(shift, 0.0)).r,
        texture(texture0, uv).g,
        texture(texture0, uv - vec2(shift, 0.0)).b
    );
}

void main()
{
    vec2 texel = 1.0 / resolution;
    vec2 uv = warp(fragTexCoord);
    vec2 screen = gl_FragCoord.xy / resolution;
    float frame = floor(time * 30.0);

    // Each scanline jitters sideways a little.
    float line = floor(gl_FragCoord.y / 2.0);
    uv.x += (hash(vec2(line, frame)) - 0.5) * jitter * glowSpread * texel.x;

    // The fringing gets worse away from the middle of the tube.
    float shift = aberration * glowSpread * texel.x * (1.0 + 1.5 * abs(screen.x - 0.5) * 2.0);

    vec3 base = scene(uv, shift);

    // Phosphor glow, a soft bloom around everything lit.
    vec3 glow = vec3(0.0);
    float total = 0.0;
    for (int x = -2; x <= 2; x++) {
        for (int y = -2; y <= 2; y++) {
            float weight = exp(-float(x * x + y * y) / 3.0);
            glow += scene(uv + vec2(x, y) * texel * glowSpread * 1.5, shift) * weight;
            total += weight;
        }
    }
    glow /= total;

    // The signal smears to the right of anything bright.
    vec3 smear = vec3(0.0);
    for (int i = 1; i <= 6; i++) {
        smear += scene(uv - vec2(float(i) * texel.x * glowSpread * 1.5, 0.0), shift) * pow(0.6, float(i));
    }

    // A faint ghost image from a reflected signal.
    vec3 ghost = scene(uv - vec2(texel.x * glowSpread * 14.0, 0.0), shift);

    vec3 colour = base * 0.5 + glow * 0.5 + smear * 0.22 + ghost * 0.05;

    // Scanlines.
    colour *= 0.78 + 0.22 * sin(gl_FragCoord.y * 3.14159 * 0.66);

    // A brighter band slowly rolls up the screen.
    float band = fract(screen.y * 0.7 - time * 0.04) - 0.5;
    colour *= 1.0 + 0.1 * exp(-band * band * 60.0);

    // A slight brightness flicker from frame to frame and some static.
    colour *= 0.95 + 0.05 * hash(vec2(frame, 0.0));
    colour += (hash(gl_FragCoord.xy + frame) - 0.5) * 0.035;

    // Darken the edges like a curved tube and round off the corners.
    vec2 centre = screen - 0.5;
    float vignette = smoothstep(0.75, 0.3, length(centre * vec2(1.0, 0.85)));
    colour *= 0.35 + 0.65 * vignette;

    vec2 edge = smoothstep(0.0, 0.01, uv) * smoothstep(0.0, 0.01, 1.0 - uv);
    colour *= edge.x * edge.y;

    finalColor = vec4(max(colour, 0.0), 1.0) * fragColor;
}
