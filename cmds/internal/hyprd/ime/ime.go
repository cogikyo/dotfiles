package ime

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	seatInterface    = "wl_seat"
	managerInterface = "zwp_input_method_manager_v2"
	doneEvent        = 5
	unavailable      = 6
	commitRequest    = 3
)

var errUnavailable = errors.New("another input method holds the seat")

// Run holds a passive input method so apps report their text-cursor boxes.
func Run(done <-chan struct{}) {
	delay := time.Second
	var last string
	for {
		start := time.Now()
		err := hold(done, func() {
			fmt.Println("hyprd ime: holding the input-method slot")
			last = ""
		})
		select {
		case <-done:
			return
		default:
		}
		if msg := err.Error(); msg != last {
			fmt.Fprintf(os.Stderr, "hyprd ime: %v; retrying\n", err)
			last = msg
		}
		if time.Since(start) > time.Minute {
			delay = time.Second
		}
		select {
		case <-done:
			return
		case <-time.After(delay):
		}
		delay = min(2*delay, time.Minute)
	}
}

func hold(done <-chan struct{}, held func()) error {
	c, err := dial()
	if err != nil {
		return err
	}
	defer c.Close()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-done:
			c.Close()
		case <-stop:
		}
	}()

	registry, sync := c.id(), c.id()
	if err := c.send(display, 1, registry); err != nil {
		return err
	}
	if err := c.send(display, 0, sync); err != nil {
		return err
	}
	var seat, manager uint32
	for synced := false; !synced; {
		e, err := c.read()
		if err != nil {
			return err
		}
		switch e.object {
		case display:
			if e.opcode == 0 {
				return displayError(e)
			}
		case registry:
			if e.opcode != 0 {
				continue
			}
			name, at, err := e.uint(0)
			if err != nil {
				return err
			}
			iface, _, err := e.string(at)
			if err != nil {
				return err
			}
			switch {
			case iface == seatInterface && seat == 0:
				seat = name
			case iface == managerInterface:
				manager = name
			}
		case sync:
			synced = true
		}
	}
	if seat == 0 {
		return errors.New("compositor offers no seat")
	}
	if manager == 0 {
		return errors.New("compositor offers no input-method manager")
	}

	seatID, managerID, im, check := c.id(), c.id(), c.id(), c.id()
	for _, req := range []struct {
		object uint32
		opcode uint16
		args   []any
	}{
		{registry, 0, []any{seat, seatInterface, uint32(1), seatID}},
		{registry, 0, []any{manager, managerInterface, uint32(1), managerID}},
		{managerID, 0, []any{seatID, im}},
		{display, 0, []any{check}},
	} {
		if err := c.send(req.object, req.opcode, req.args...); err != nil {
			return err
		}
	}
	var serial uint32
	for {
		e, err := c.read()
		if err != nil {
			return err
		}
		switch {
		case e.object == display && e.opcode == 0:
			return displayError(e)
		case e.object == im && e.opcode == doneEvent:
			serial++
			// Empty commits let Hyprland acknowledge text-input state.
			// Chromium withholds cursor boxes until that acknowledgement arrives.
			if err := c.send(im, commitRequest, serial); err != nil {
				return err
			}
		case e.object == im && e.opcode == unavailable:
			return errUnavailable
		case e.object == check:
			held()
		}
	}
}

func displayError(e event) error {
	object, at, err := e.uint(0)
	if err != nil {
		return err
	}
	code, at, err := e.uint(at)
	if err != nil {
		return err
	}
	msg, _, err := e.string(at)
	if err != nil {
		return err
	}
	return fmt.Errorf("wayland error on object %d (code %d): %s", object, code, msg)
}
