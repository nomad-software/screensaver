#version 330

// Reef by @Xor
// https://fragcoord.xyz/s/rk14ca7w

uniform vec2 u_resolution;
uniform float u_time;

out vec4 finalColor;

void main()
{
    // vec2 uv = (2.0 * gl_FragCoord.xy - u_resolution.xy)
    //         / u_resolution.y;
	vec2 uv = (2.0 * vec2(gl_FragCoord.x, u_resolution.y - gl_FragCoord.y)
        - u_resolution.xy)
        / u_resolution.y;

    // vec3 ro = vec3(0.0, 0.0, -35.0);
    // vec3 rd = normalize(vec3(uv * 0.55, 1.0));
	vec3 ro = vec3(0.0, -2.0, -20.0);

	float camAngle = 0.10;

	mat3 camRot = mat3(
		1.0, 0.0, 0.0,
		0.0, cos(camAngle), -sin(camAngle),
		0.1, sin(camAngle),  cos(camAngle)
	);

	vec3 rd = normalize(camRot * vec3(uv * 0.50, 1.5));

    float z = 0.0;
    float d = 0.0;

    vec4 colour = vec4(0.0);

    for (int i = 1; i <= 50; i++)
    {
        vec3 p = ro + rd * z;

        for (int j = 1; j <= 9; j++)
        {
            float fj = float(j);

            p += 0.4 *
                 sin(p.yzx * fj - z + u_time + float(i))
                 / fj
                 + 0.5;
        }

        d = length(
                vec4(
                    abs(p.y + p.z * 0.5),
                    sin(p - z) / 7.0
                )
            )
            / (4.0 + z * z / 100.0);

        float safeD = max(d, 0.001);
        float safeZ = max(z, 0.01);

        colour +=
            (0.9 + sin(float(i) * 0.1 - vec4(6.0, 1.0, 2.0, 0.0)))
            / (safeD * safeD * safeZ)
            + safeD * safeZ / vec4(4.0, 2.0, 1.0, 0.0001);

        z += d;

        if (z > 30.0)
            break;
    }

    finalColor = tanh(colour / 2000.0);
}
