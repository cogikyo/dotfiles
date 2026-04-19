/* Geometry */
#define C_RADIUS 90
#define C_LINE 0
#define OUTLINE #6380ec
#define NBAR 80
#define BAR_WIDTH 3.5
#define BAR_OUTLINE OUTLINE
#define BAR_OUTLINE_WIDTH 0

/* Appearance */
#define AMPLIFY 180
#define COLOR (#6380ec * ((d / 180) + 0.69))
#define ROTATE (PI/1.75)
#define INVERT 0

/* Aliasing -- requires xroot transparency for alpha blending */
#define BAR_ALIAS_FACTOR 1.2
#define C_ALIAS_FACTOR 1.8

/* Position offset */
#define CENTER_OFFSET_Y 0
#define CENTER_OFFSET_X 0

/* Override smooth_parameters.glsl */
#request setgravitystep 11.0
#request setsmoothfactor 0.027
