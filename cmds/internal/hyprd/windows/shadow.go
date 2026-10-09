package windows

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

// Parked returns the member of st waiting on ShadowWorkspace, or nil.
//
// Roles come from live placement, because windows can move without hyprd updating the record.
func Parked(st *state.ThreeBodyState, clients []hypr.Window) *hypr.Window {
	i := slices.IndexFunc(clients, func(c hypr.Window) bool {
		return c.Workspace.Name == ShadowWorkspace && st.Has(c.Address)
	})
	if i < 0 {
		return nil
	}
	return &clients[i]
}

// Park floats w at its current box on ShadowWorkspace and fits it to wsID's slave box.
func Park(h *hypr.Client, w hypr.Window, wsID int) error {
	if err := h.Park(w.Address, ShadowWorkspace, w.At, w.Size); err != nil {
		return err
	}
	return Fit(h, w.Address, wsID)
}

// Find returns the client with address, or nil.
func Find(clients []hypr.Window, address string) *hypr.Window {
	i := slices.IndexFunc(clients, func(c hypr.Window) bool { return c.Address == address })
	if i < 0 {
		return nil
	}
	return &clients[i]
}

// Fit sizes a parked window to wsID's current slave box, so the hidden app reflows before it is shown.
func Fit(h *hypr.Client, address string, wsID int) error {
	clients, err := h.Clients()
	if err != nil {
		return err
	}
	w := Find(clients, address)
	slaves := GetSlaves(Tiled(clients, wsID))
	if w == nil || !w.Floating || w.Workspace.Name != ShadowWorkspace || len(slaves) == 0 {
		return nil
	}
	return h.Fit(address, slaves[0].At, slaves[0].Size)
}

// Unpark tiles a parked window on wsID, as its slave or as master when wsID is empty.
func Unpark(h *hypr.Client, address string, wsID int) error {
	return h.Unpark(address, strconv.Itoa(wsID))
}

// SwapSlot trades a parked window with a tiled one on wsID and focuses it.
func SwapSlot(h *hypr.Client, incoming, outgoing string, wsID int) error {
	if incoming == outgoing {
		return fmt.Errorf("swap %s with itself", incoming)
	}
	clients, err := h.Clients()
	if err != nil {
		return err
	}
	if w := Find(clients, incoming); w != nil && !w.Floating {
		if err := Park(h, *w, wsID); err != nil {
			return err
		}
	}
	return h.SwapSlot(incoming, outgoing)
}

// Refit restores the parked-shadow invariant for wsID's three-body: floating on ShadowWorkspace at the slave box.
func Refit(h *hypr.Client, s *state.State, wsID int) error {
	st := s.GetThreeBody(wsID)
	if st == nil || s.GetMonocle(wsID) != nil {
		return nil
	}
	clients, err := h.Clients()
	if err != nil {
		return err
	}
	shadow := Parked(st, clients)
	if shadow == nil {
		return nil
	}
	if !shadow.Floating {
		return Park(h, *shadow, wsID)
	}
	slaves := GetSlaves(Tiled(clients, wsID))
	if len(slaves) == 0 || slaves[0].At == shadow.At && slaves[0].Size == shadow.Size {
		return nil
	}
	return h.Fit(shadow.Address, slaves[0].At, slaves[0].Size)
}

// RefitAll runs Refit on every workspace with a three-body.
func RefitAll(h *hypr.Client, s *state.State) error {
	var errs []error
	for wsID := range s.AllThreeBody() {
		if err := Refit(h, s, wsID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
