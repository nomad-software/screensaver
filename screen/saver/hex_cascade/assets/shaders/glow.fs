#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 texelSize;
uniform float glowRadius;

out vec4 finalColor;

const int taps = 32;
const float goldenAngle = 2.39996;

void main()
{
    // Four bilinear samples half a texel off each way soften the hard edges
    // of the tiles.
    vec2 h = texelSize * 0.5;
    vec3 base = texture(texture0, fragTexCoord).rgb * 0.5;
    base += texture(texture0, fragTexCoord + vec2( h.x,  h.y)).rgb * 0.125;
    base += texture(texture0, fragTexCoord + vec2(-h.x,  h.y)).rgb * 0.125;
    base += texture(texture0, fragTexCoord + vec2( h.x, -h.y)).rgb * 0.125;
    base += texture(texture0, fragTexCoord + vec2(-h.x, -h.y)).rgb * 0.125;

    // Bright tiles glow, sampled in a spiral around each pixel.
    vec3 glow = vec3(0.0);
    float total = 0.0;

    for (int i = 0; i < taps; i++)
    {
        float f = (float(i) + 0.5) / float(taps);
        float a = float(i) * goldenAngle;
        vec2 offset = vec2(cos(a), sin(a)) * sqrt(f) * glowRadius * texelSize;

        vec3 s = texture(texture0, fragTexCoord + offset).rgb;
        float bright = max(max(s.r, s.g), s.b);
        float weight = 1.0 - f;

        glow += s * smoothstep(0.8, 1.0, bright) * weight;
        total += weight;
    }

    glow /= total;

    // Darken the corners.
    vec2 uv = fragTexCoord - 0.5;
    float vignette = mix(0.55, 1.0, smoothstep(0.85, 0.25, length(uv)));

    finalColor = vec4((base + glow * 0.9) * vignette, 1.0) * fragColor;
}
