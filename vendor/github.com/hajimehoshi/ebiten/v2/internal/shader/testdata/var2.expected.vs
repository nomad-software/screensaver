vec4 F0(in vec2 l0);

vec4 F0(in vec2 l0) {
	vec4 l1 = vec4(0);
	vec4 l2 = vec4(0);
	l1 = vec4(l0, 0.0, 1.0);
	(l1).x = (l1).x;
	l2 = l1;
	return l2;
}
