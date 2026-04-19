/* Bar geometry */
#define C_LINE 1
#define BAR_WIDTH 2
#define BAR_GAP 2
#define BAR_OUTLINE #262626
#define BAR_OUTLINE_WIDTH 0

/* Appearance */
#define AMPLIFY 356
#define USE_ALPHA 0
#define GRADIENT_POWER 69
#define GRADIENT (d / GRADIENT_POWER + 0.69)
#define COLOR (#6380ec * GRADIENT)

/* Orientation */
#define DIRECTION 1      /* 0 = inward, 1 = outward */
#define INVERT 0
#define FLIP 0
#define MIRROR_YX 1      /* renders on left side; combine with FLIP 1 for right */

/* Edge falloff -- taper bars at spectrum ends */
#define EDGE_FALLOFF 1
#define EDGE_START 0.15  /* low-freq fade-in portion (0.0-1.0) */
#define EDGE_END 0.25    /* high-freq fade-out portion (0.0-1.0) */
#define EDGE_POWER 2.2   /* curve steepness (1.0 = linear, 2.0 = quadratic) */
