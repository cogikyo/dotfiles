OUT ?= caret.so
CXXFLAGS ?= -O2
PKGS ?= hyprland pixman-1 libdrm

$(OUT): caret.cpp
	$(CXX) -shared -fPIC -std=c++2b -Wall -Wextra $(CXXFLAGS) $(CPPFLAGS) $$(pkg-config --cflags $(PKGS)) caret.cpp -o $@

.PHONY: $(OUT)
