#version 330

// 3D Fire Golf by @Xor
// https://fragcoord.xyz/s/3uxnauw2

uniform vec2 u_resolution;
uniform float u_time;

out vec4 finalColor;

void main()
{
    vec2 uv = (2.0 * gl_FragCoord.xy - u_resolution.xy)
            / u_resolution.y;

    // Camera position.
    // Moving this further back zooms out without changing FOV.
    vec3 ro = vec3(0.0, 0.0, -10.0);

    // Camera direction / FOV.
    vec3 rd = normalize(vec3(uv * 0.55, 1.0));

    float t = 0.0;
    vec4 colour = vec4(0.0);

    for (int i = 0; i < 70; i++)
    {
        vec3 p = ro + rd * t;

        p.z += 5.0 + cos(u_time);

        float a = u_time + p.y / 4.0;

        p.xz *= mat2(
            cos(a),
            cos(a + 8.0),
            cos(a + 5.0),
            cos(a)
        );

        float d = 2.0;

        for (int j = 0; j < 6; j++)
        {
            d /= 0.8;

            p += cos(
                // (p.yzx - vec3(u_time, 0.0, 0.0) * 8.0)
                (p.yzx - vec3(u_time, 0.0, 0.0)) // slow it down.
                * d + u_time
            ) / d;
        }

        d = 0.01
          + abs(length(p.xz) + p.y * 0.3 - 1.0) / 9.0;

        t += d;

        colour += (
            sin(
                p.y / 2.0
                - vec4(0.0, 1.0, 2.0, 0.0)
            ) + 1.1
        ) / d;

        if (t > 30.0)
            break;
    }

    finalColor = tanh(colour / 1000.0);
}
