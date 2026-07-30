#version 330

// star grid by @dragonfly
// https://fragcoord.xyz/s/s386lake

uniform vec2 u_resolution;
uniform float u_time;

out vec4 finalColor;

float sd_segment(vec2 p, vec2 a, vec2 b)
{
    vec2 pa = p - a;
    vec2 ba = b - a;
    float h = clamp(dot(pa, ba) / dot(ba, ba), 0.0, 1.0);
    return length(pa - ba * h);
}

float hash21(vec2 p)
{
    p = fract(p * vec2(131.088, 237.068));
    p += dot(p, p + 34.5);
    return fract(p.x * p.y);
}

vec2 hash22(vec2 p)
{
    float n = hash21(p);
    return vec2(n, fract(n * 321.324));
}

vec2 get_pos(vec2 id)
{
    vec2 n = hash22(id);
    n = sin(n * u_time);
    return n * 0.5 + 0.5 + id;
}

float line(vec2 p, vec2 p1, vec2 p2)
{
    float d = sd_segment(p, p1, p2);
    return smoothstep(0.015, 0.0, d)
         * smoothstep(1.2, 0.8, length(p1 - p2));
}

float layer(vec2 uv)
{
    vec2 id = floor(uv);

    vec2 p[9];
    int idx = 0;

    float c = 0.0;

    for(int y = -1; y <= 1; y++)
    {
        for(int x = -1; x <= 1; x++)
        {
            vec2 offset = vec2(float(x), float(y));
            vec2 pp = get_pos(id + offset);

            p[idx++] = pp;

            float d = length(uv - pp);

            c += (0.01 / max(d, 0.001))
               * smoothstep(0.9, 0.3, d);
        }
    }

    c += line(uv, p[0], p[4]);
    c += line(uv, p[1], p[4]);
    c += line(uv, p[2], p[4]);
    c += line(uv, p[3], p[4]);
    c += line(uv, p[5], p[4]);
    c += line(uv, p[6], p[4]);
    c += line(uv, p[7], p[4]);
    c += line(uv, p[8], p[4]);

    c += line(uv, p[1], p[3]);
    c += line(uv, p[1], p[5]);
    c += line(uv, p[3], p[7]);
    c += line(uv, p[5], p[7]);

    return c;
}

mat2 rotate2d(float a)
{
    float s = sin(a);
    float c = cos(a);
    return mat2(c, -s,
                s,  c);
}

void main()
{
    vec2 uv =
        (gl_FragCoord.xy - 0.5 * u_resolution)
        / min(u_resolution.x, u_resolution.y);

    uv *= rotate2d(u_time * 0.15);

    float brightness = 0.0;

    const float stepSize = 0.2;

    // No mouse uniform in raylib version.
    vec2 mouseOffset = vec2(0.0);

    for(float i = 0.0; i < 1.0; i += stepSize)
    {
        float depth = fract(i - u_time * 0.2);

        float scale = mix(0.4, 8.0, depth);

        brightness +=
            layer(uv * scale + i * 15.0 + mouseOffset)
            * smoothstep(0.0, 0.2, depth)
            * smoothstep(1.0, 0.8, depth);
    }

    vec3 colour =
        sin(vec3(0.5, 0.7, 0.9) * u_time)
        * 0.5
        + vec3(0.7, 0.2, 0.8);

    finalColor = vec4(colour * brightness, 1.0);
}
