#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 resolution; // Screen size in pixels.
uniform float time;
uniform float ghost; // The strength of the signal reflections, from 0 to 1.

out vec4 finalColor;

const vec3 phosphor = vec3(0.55, 1.0, 0.65); // The colour of noise on the tube.

float hash(vec2 p)
{
    return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

// Sample the picture with the red and blue guns misaligned sideways. Anything
// outside the picture is the unlit tube.
vec3 scene(vec2 uv, float shift)
{
    if (uv.x < 0.0 || uv.x > 1.0) {
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
    vec2 uv = fragTexCoord;
    float frame = mod(floor(time * 60.0), 1000.0);

    // The lines never sit quite still.
    float line = floor(uv.y * resolution.y / 2.0);
    float wobble = hash(vec2(line, frame)) - 0.5;

    vec2 pos = vec2(uv.x + wobble * 1.5 * texel.x, uv.y);

    // The colour fringing gets worse while the signal is ghosting.
    float shift = (1.0 + ghost * 6.0) * texel.x;

    vec3 colour = scene(pos, shift);

    // The signal smears to the right of anything bright.
    for (int i = 1; i <= 4; i++) {
        colour += scene(pos - vec2(float(i) * 2.0 * texel.x, 0.0), shift) * pow(0.5, float(i)) * 0.3;
    }

    // Reflected signals leave ghosts trailing off to the right.
    float echo = ghost * (0.75 + 0.25 * hash(vec2(frame, 7.0)));
    colour += scene(pos - vec2(0.012 + 0.01 * echo, 0.0), shift) * echo * 0.55;
    colour += scene(pos - vec2(0.03 + 0.025 * echo, 0.0), shift) * echo * 0.3;

    // Scanlines.
    colour *= 0.82 + 0.18 * sin(gl_FragCoord.y * 2.0944);

    // Mains hum, a dark bar drifting up the screen.
    float hum = 0.5 + 0.5 * sin((uv.y - time * 1.5) * 6.2832);
    colour *= 1.0 - 0.07 * hum;

    // The brightness flickers.
    colour *= 0.95 + 0.05 * hash(vec2(frame, 1.0));

    // A little grain.
    float grain = hash(gl_FragCoord.xy + frame);
    colour += phosphor * (grain - 0.5) * 0.04;

    // The tube is a little darker towards its edges.
    vec2 centre = uv - 0.5;
    float vignette = smoothstep(0.8, 0.3, length(centre * vec2(1.0, 0.85)));
    colour *= 0.6 + 0.4 * vignette;

    finalColor = vec4(max(colour, 0.0), 1.0) * fragColor;
}
