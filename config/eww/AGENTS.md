# eww

Widget definitions and SCSS for the eww UI.
See the [ewwd guide](../../cmds/cmd/ewwd/README.md) for signal and action behavior.

## Layout

| Path       | Holds                             |
| ---------- | --------------------------------- |
| `eww.yuck` | Widget include entrypoint         |
| `yuck/`    | Windows, widgets, listeners       |
| `eww.scss` | Style import entrypoint           |
| `styles/`  | Widget styles and `_palette.scss` |

## Wiring

- Add widget includes through `eww.yuck` and style imports through `eww.scss`.
- Listeners use `hyprd subscribe` or `ewwd subscribe` and extract the JSON envelope's `.data` with unbuffered jq.
- Signal providers live in `../../cmds/internal/ewwd/providers/`; workspace signals come from hyprd.
- Names in `../../cmds/config/ewwd.yaml` → `windows` must match Yuck `defwindow` names.
