/* Smoothing and FFT parameters.
   Values here can be overridden in per-module config files. */

/* Weighting formula: circular | sinusoidal | linear */
#define ROUND_FORMULA sinusoidal

/* Sampling mode: average | maximum | hybrid */
#define SAMPLE_MODE average
#define SAMPLE_HYBRID_WEIGHT 0.65 /* (0,1) -- higher favors averaged results; hybrid only */

#define SAMPLE_SCALE 8   /* lower = more space for low frequencies */
#define SAMPLE_RANGE 0.9 /* portion of FFT output to display (0.0-1.0) */

#request setfftscale 10.2
#request setfftcutoff 0.3
#request setavgframes 6
#request setavgwindow true

/* val -= gravitystep * seconds_per_update */
#request setgravitystep 7

/* [0.0, 1.0) -- larger = more smoothing, but more expensive */
#request setsmoothfactor 0.027

/* Use separate render pass for audio smoothing (faster on most hardware) */
#request setsmoothpass true
