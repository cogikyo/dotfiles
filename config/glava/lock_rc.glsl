#request mod lock

/* Window: the lock passes setgeometry with -r */
#request setfloating  true
#request setdecorated false
#request setfocused   false
#request setmaximized false
#request setopacity "native"
#request settitle "GLava lock"
#request setbg 00000000

/* X11 */
#request setxwintype "dock"
#request setclickthrough true

/* Audio */
#request setmirror false
#request setsource "auto"
#request setsamplesize 1024
#request setbufsize 4096
#request setsamplerate 22050

/* Rendering */
#request setversion 3 3
#request setshaderversion 330
#request setswap 1
#request setinterpolate true
#request setframerate 60
#request setfullscreencheck false
#request setprintframes false
