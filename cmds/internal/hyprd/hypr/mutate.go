package hypr

// mutate.go is the typed Hyprland 0.56+ mutation surface.
// Mutations go through `eval` Lua (`hl.dispatch` / `hl.config` / …), not legacy dispatch/keyword.

import (
	"fmt"
	"strings"
)

// eval sends `eval <lua>` on the request socket.
// Success is a response of exactly "ok" after trimming whitespace.
func (c *Client) eval(op, lua string) error {
	resp, err := c.Request("eval " + lua)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if got := strings.TrimSpace(string(resp)); got != "ok" {
		return fmt.Errorf("%s: %s", op, got)
	}
	return nil
}

// luaQuote returns a Lua double-quoted string literal.
func luaQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func luaBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func windowAddress(address string) string {
	return "address:" + address
}

// FocusWorkspace focuses workspace id.
func (c *Client) FocusWorkspace(id int) error {
	return c.eval("FocusWorkspace", fmt.Sprintf(
		"hl.dispatch(hl.dsp.focus({ workspace = %d }))", id,
	))
}

// FocusWindow focuses the window at the raw hex address.
func (c *Client) FocusWindow(address string) error {
	return c.eval("FocusWindow", fmt.Sprintf(
		"hl.dispatch(hl.dsp.focus({ window = %s }))",
		luaQuote(windowAddress(address)),
	))
}

// MoveActiveToWorkspace moves the active window to workspace id.
// follow=false is the old silent move.
func (c *Client) MoveActiveToWorkspace(id int, follow bool) error {
	return c.eval("MoveActiveToWorkspace", fmt.Sprintf(
		"hl.dispatch(hl.dsp.window.move({ workspace = %d, follow = %s }))",
		id, luaBool(follow),
	))
}

// MoveWindowToWorkspace moves window address to workspace (selector string).
// workspace may be "3", "special:shadow", or a name. follow=false is silent.
func (c *Client) MoveWindowToWorkspace(address string, workspace string, follow bool) error {
	return c.eval("MoveWindowToWorkspace", fmt.Sprintf(
		"hl.dispatch(hl.dsp.window.move({ workspace = %s, follow = %s, window = %s }))",
		luaQuote(workspace), luaBool(follow), luaQuote(windowAddress(address)),
	))
}

// ToggleFloatActive toggles floating on the active window.
func (c *Client) ToggleFloatActive() error {
	// Hyprland default config uses action = "toggle"; bare float() is ambiguous.
	return c.eval("ToggleFloatActive",
		`hl.dispatch(hl.dsp.window.float({ action = "toggle" }))`,
	)
}

// ResizeActiveExact resizes the active window to exact pixel size w×h.
func (c *Client) ResizeActiveExact(w, h int) error {
	return c.eval("ResizeActiveExact", fmt.Sprintf(
		"hl.dispatch(hl.dsp.window.resize({ x = %d, y = %d }))", w, h,
	))
}

// MoveActiveRelative moves the active window by (dx, dy) pixels.
func (c *Client) MoveActiveRelative(dx, dy int) error {
	return c.eval("MoveActiveRelative", fmt.Sprintf(
		"hl.dispatch(hl.dsp.window.move({ x = %d, y = %d, relative = true }))",
		dx, dy,
	))
}

// MoveWindowDirection moves the active window in dir ("left"|"right"|"up"|"down").
func (c *Client) MoveWindowDirection(dir string) error {
	return c.eval("MoveWindowDirection", fmt.Sprintf(
		"hl.dispatch(hl.dsp.window.move({ direction = %s }))",
		luaQuote(dir),
	))
}

// CenterActive centers the active window.
func (c *Client) CenterActive() error {
	return c.eval("CenterActive", `hl.dispatch(hl.dsp.window.center())`)
}

// CloseWindow closes the window at address.
func (c *Client) CloseWindow(address string) error {
	return c.eval("CloseWindow", fmt.Sprintf(
		"hl.dispatch(hl.dsp.window.close({ window = %s }))",
		luaQuote(windowAddress(address)),
	))
}

// Exec runs cmd via Hyprland's exec dispatcher.
func (c *Client) Exec(cmd string) error {
	return c.eval("Exec", fmt.Sprintf(
		"hl.dispatch(hl.dsp.exec_cmd(%s))", luaQuote(cmd),
	))
}

// ExecOnWorkspace runs cmd, placing the new window on workspace.
// silent mirrors the old "[workspace N silent]" rule semantics.
func (c *Client) ExecOnWorkspace(cmd string, workspace int, silent bool) error {
	ws := fmt.Sprintf("%d", workspace)
	if silent {
		ws += " silent"
	}
	return c.eval("ExecOnWorkspace", fmt.Sprintf(
		"hl.dispatch(hl.dsp.exec_cmd(%s, { workspace = %s }))",
		luaQuote(cmd), luaQuote(ws),
	))
}

// Submap enters named submap; "reset" leaves the current submap.
func (c *Client) Submap(name string) error {
	return c.eval("Submap", fmt.Sprintf(
		"hl.dispatch(hl.dsp.submap(%s))", luaQuote(name),
	))
}

// ToggleSpecialWorkspace toggles the named special workspace.
func (c *Client) ToggleSpecialWorkspace(name string) error {
	return c.eval("ToggleSpecialWorkspace", fmt.Sprintf(
		"hl.dispatch(hl.dsp.workspace.toggle_special(%s))", luaQuote(name),
	))
}

// LayoutMsg sends a layoutmsg string (e.g. "swapwithmaster master").
func (c *Client) LayoutMsg(msg string) error {
	return c.eval("LayoutMsg", fmt.Sprintf(
		"hl.dispatch(hl.dsp.layout(%s))", luaQuote(msg),
	))
}

// MoveCursor warps the cursor to absolute (x, y).
func (c *Client) MoveCursor(x, y int) error {
	return c.eval("MoveCursor", fmt.Sprintf(
		"hl.dispatch(hl.dsp.cursor.move({ x = %d, y = %d }))", x, y,
	))
}

// SetAccent sets active border and shadow colors (rgba(...) strings).
func (c *Client) SetAccent(border, shadow string) error {
	return c.eval("SetAccent", fmt.Sprintf(
		"hl.config({ general = { col = { active_border = %s } }, decoration = { shadow = { color = %s } } })",
		luaQuote(border), luaQuote(shadow),
	))
}

// SetOuterGaps sets general.gaps_out (top, right, bottom, left).
func (c *Client) SetOuterGaps(top, right, bottom, left int) error {
	return c.eval("SetOuterGaps", fmt.Sprintf(
		"hl.config({ general = { gaps_out = { top = %d, right = %d, bottom = %d, left = %d } } })",
		top, right, bottom, left,
	))
}

// SetWorkspaceAnim sets the workspaces animation style ("slide"|"slidevert").
func (c *Client) SetWorkspaceAnim(style string) error {
	return c.eval("SetWorkspaceAnim", fmt.Sprintf(
		`hl.animation({ leaf = "workspaces", enabled = true, speed = 3, bezier = "default", style = %s })`,
		luaQuote(style),
	))
}

// AddFadeRule adds a dynamic window rule with animation = "fade".
// initialTitle may be empty (class-only match).
func (c *Client) AddFadeRule(class, initialTitle string) error {
	match := "class = " + luaQuote(class)
	if initialTitle != "" {
		match += ", initial_title = " + luaQuote(initialTitle)
	}
	return c.eval("AddFadeRule", fmt.Sprintf(
		`hl.window_rule({ match = { %s }, animation = "fade" })`, match,
	))
}
