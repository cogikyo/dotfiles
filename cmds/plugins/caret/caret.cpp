#include <hyprland/src/plugins/PluginAPI.hpp>
#include <hyprland/src/managers/input/InputManager.hpp>
#include <hyprland/src/managers/input/TextInput.hpp>
#include <hyprland/src/desktop/view/WLSurface.hpp>

#include <format>
#include <string>

namespace {

    std::string none(const char* reason) {
        return std::format(R"({{"found":false,"reason":"{}"}})", reason);
    }

    std::string caret() {
        if (!g_pInputManager)
            return none("no input manager");

        const auto TI = g_pInputManager->m_relay.getFocusedTextInput();
        if (!TI)
            return none("no focused text input");
        if (!TI->isEnabled())
            return none("text input disabled");
        if (!TI->hasCursorRectangle())
            return none("no cursor rectangle");

        const auto SURFACE = TI->focusedSurface();
        if (!SURFACE)
            return none("no focused surface");

        const auto OWNER = Desktop::View::CWLSurface::fromResource(SURFACE);
        if (!OWNER)
            return none("surface has no owner");

        const auto PARENT = OWNER->getSurfaceBoxGlobal();
        if (!PARENT)
            return none("surface has no global box");

        const auto BOX = TI->cursorBox();
        if (BOX.w <= 0 && BOX.h <= 0)
            return none("empty cursor rectangle");

        return std::format(R"({{"found":true,"x":{},"y":{},"w":{},"h":{}}})", PARENT->x + BOX.x, PARENT->y + BOX.y, BOX.w, BOX.h);
    }

    std::string reply() {
        try {
            return caret();
        } catch (...) { return none("internal error"); }
    }

    template <typename F>
    struct SCommandOf;

    template <typename R, typename C>
    struct SCommandOf<R (*)(HANDLE, C)> {
        using type = C;
    };

    template <typename C>
    void registerCommand(HANDLE handle) {
        C command;
        command.name = "caret";
        if constexpr (requires(C c) { c.handler; })
            command.handler = [](const auto&) { return reply(); };
        else
            command.fn = [](auto, const std::string&) { return reply(); };
        HyprlandAPI::registerHyprCtlCommand(handle, command);
    }

}

APICALL EXPORT std::string PLUGIN_API_VERSION() {
    return HYPRLAND_API_VERSION;
}

APICALL EXPORT PLUGIN_DESCRIPTION_INFO PLUGIN_INIT(HANDLE handle) {
    if (std::string{__hyprland_api_get_hash()} != __hyprland_api_get_client_hash()) {
        HyprlandAPI::addNotification(handle, "[caret] built for another Hyprland; not loaded", CHyprColor{1.0, 0.2, 0.2, 1.0}, 5000);
        throw std::runtime_error("[caret] version mismatch");
    }

    registerCommand<SCommandOf<decltype(&HyprlandAPI::registerHyprCtlCommand)>::type>(handle);

    return {"caret", "hyprctl caret: the focused text input's cursor box in global logical coordinates", "cullyn", "1"};
}

APICALL EXPORT void PLUGIN_EXIT() {}
