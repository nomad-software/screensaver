#version 330

in vec2 fragTexCoord;

out vec4 fragColor;

uniform sampler2D texture0;
uniform vec4 colDiffuse;

vec4 brightness = vec4(2.5, 2.0, 2.5, 1.25);

void main()
{
    vec4 texColor = texture(texture0, fragTexCoord);

    fragColor = texColor * brightness;
}
