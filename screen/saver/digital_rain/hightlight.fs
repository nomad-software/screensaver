#version 330 core

uniform sampler2D texture0;

in vec2 fragCoord;
out vec4 fragColor;

void main()
{
    vec2 uv = fragCoord / vec2(2560, 1440);
    vec4 existingColor = texture(texture0, fragCoord);

    if (uv.x > 0.4 && uv.x < 0.6 && uv.y > 0.3 && uv.y < 0.7)
    {
        // vec3 brightenedColor = existingColor.rgb * 2.0;
        // fragColor = vec4(brightenedColor, 1.0);
        fragColor = existingColor;
    }
    else
    {
        fragColor = existingColor;
    }
}
