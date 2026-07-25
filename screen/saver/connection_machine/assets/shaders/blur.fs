#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 texelSize;
uniform float blurAmount;

out vec4 finalColor;

void main()
{
    vec4 colour = vec4(0.0);

    colour += texture(texture0, fragTexCoord + vec2(-4.0, 0.0) * texelSize * blurAmount) * 0.05;
    colour += texture(texture0, fragTexCoord + vec2(-3.0, 0.0) * texelSize * blurAmount) * 0.09;
    colour += texture(texture0, fragTexCoord + vec2(-2.0, 0.0) * texelSize * blurAmount) * 0.12;
    colour += texture(texture0, fragTexCoord + vec2(-1.0, 0.0) * texelSize * blurAmount) * 0.15;
    colour += texture(texture0, fragTexCoord) * 0.18;
    colour += texture(texture0, fragTexCoord + vec2(1.0, 0.0) * texelSize * blurAmount) * 0.15;
    colour += texture(texture0, fragTexCoord + vec2(2.0, 0.0) * texelSize * blurAmount) * 0.12;
    colour += texture(texture0, fragTexCoord + vec2(3.0, 0.0) * texelSize * blurAmount) * 0.09;
    colour += texture(texture0, fragTexCoord + vec2(4.0, 0.0) * texelSize * blurAmount) * 0.05;

    finalColor = colour * fragColor;
}
