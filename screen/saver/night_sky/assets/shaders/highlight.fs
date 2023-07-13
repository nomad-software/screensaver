#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

out vec4 finalColor;

uniform sampler2D texture0;
uniform vec4 colDiffuse;

vec4 brightness = vec4(2.5, 2.0, 2.5, 1.25);

void main()
{
    vec4 texelColor = texture(texture0, fragTexCoord);

    finalColor = texelColor * brightness;
}
