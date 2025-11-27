/* Center line thickness (pixels) */
#define C_LINE 1
/* Width (in pixels) of each bar */
#define BAR_WIDTH 2
/* Width (in pixels) of each bar gap */
#define BAR_GAP 2
/* Outline color */
#define BAR_OUTLINE #262626
/* Outline width (in pixels, set to 0 to disable outline drawing) */
#define BAR_OUTLINE_WIDTH 0
/* Amplify magnitude of the results each bar displays */
#define AMPLIFY 220
/* Whether the current settings use the alpha channel;
   enabling this is required for alpha to function
   correctly on X11 with `"native"` transparency. */
#define USE_ALPHA 0
/* How strong the gradient changes */
#define GRADIENT_POWER 69
/* Bar color changes with height */
#define GRADIENT (d / GRADIENT_POWER + 0.69)
/* Bar color */
#define COLOR (#6380ec * GRADIENT)
/* Direction that the bars are facing, 0 for inward, 1 for outward */
#define DIRECTION 1
/* Whether to switch left/right audio buffers */
#define INVERT 0
/* Whether to flip the output vertically */
#define FLIP 0
/* Whether to mirror output along `Y = X`, causing output to render on the left side of the window */
/* Use with `FLIP 1` to render on the right side */
#define MIRROR_YX 1

/* Edge falloff - smoothly taper bars at spectrum ends */
#define EDGE_FALLOFF 1
/* Portion of low frequencies to fade in (0.0 - 1.0) */
#define EDGE_START 0.15
/* Portion of high frequencies to fade out (0.0 - 1.0) */
#define EDGE_END 0.25
/* Curve steepness (higher = sharper falloff, 1.0 = linear, 2.0 = quadratic) */
#define EDGE_POWER 2.2
