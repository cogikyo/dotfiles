package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dotfiles/cmds/internal/config"
)

const (
	voiceStale = 25 * time.Second
	voiceRetry = 5 * time.Second
)

type VoiceState struct {
	Mode string `json:"mode"`
}

type Voice struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	last      string
	published bool
}

func NewVoice(state StateSetter) Provider {
	return &Voice{state: state, done: make(chan struct{})}
}

func (v *Voice) Name() string {
	return "voice"
}

func (v *Voice) Start(ctx context.Context, notify func(data any)) error {
	v.active = true
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	vagus := filepath.Join(config.CogikyoBin, "vagus")
	if _, err := os.Stat(vagus); err != nil {
		v.publish(notify, "absent")
		select {
		case <-ctx.Done():
		case <-v.done:
		}
		return nil
	}

	host, _ := os.Hostname()
	events := make(chan voiceEvent)
	go tailVoice(ctx, vagus, host, events)
	v.publish(notify, "down")

	var hearing string
	var hush *bool
	held := thalamusHeld(ctx, host)
	hearingStale := time.NewTimer(voiceStale)
	defer hearingStale.Stop()
	hushStale := time.NewTimer(voiceStale)
	defer hushStale.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-v.done:
			return nil
		case e := <-events:
			switch e.Topic {
			case "cochlea.hearing":
				hearing = e.Body.Mode
				hearingStale.Reset(voiceStale)
			case "thalamus.hush":
				hush = e.Body.On
				hushStale.Reset(voiceStale)
			}
		case <-hearingStale.C:
			hearing = ""
		case <-hushStale.C:
			hush = nil
			held = thalamusHeld(ctx, host)
			hushStale.Reset(voiceStale)
		}
		v.publish(notify, voiceMode(hearing, hush, held))
	}
}

func voiceMode(hearing string, hush *bool, held bool) string {
	switch {
	case hearing == "":
		return "down"
	case hearing != "listening":
		return hearing
	case hush == nil && held:
		return "thalamus-held"
	case hush == nil:
		return "thalamus-down"
	case *hush:
		return "hushed"
	}
	return "listening"
}

func thalamusHeld(ctx context.Context, host string) bool {
	ctx, cancel := context.WithTimeout(ctx, voiceRetry)
	defer cancel()
	out, err := exec.CommandContext(ctx, filepath.Join(config.CogikyoBin, "cogictl"), "organ", "status", host, "--json").Output()
	if err != nil {
		return false
	}
	var organs []struct {
		Organ string `json:"organ"`
		Hold  string `json:"hold"`
	}
	if json.Unmarshal(out, &organs) != nil {
		return false
	}
	for _, o := range organs {
		if o.Organ == "thalamus" {
			return o.Hold != ""
		}
	}
	return false
}

func (v *Voice) Stop() error {
	if v.active {
		close(v.done)
		v.active = false
	}
	return nil
}

func (v *Voice) publish(notify func(data any), mode string) {
	if v.published && v.last == mode {
		return
	}
	v.last = mode
	v.published = true
	state := VoiceState{Mode: mode}
	v.state.Set("voice", &state)
	notify(&state)
}

type voiceEvent struct {
	Topic  string `json:"topic"`
	Source struct {
		Host string `json:"host"`
	} `json:"source"`
	Body struct {
		Mode string `json:"mode"`
		On   *bool  `json:"on"`
	} `json:"body"`
}

func tailVoice(ctx context.Context, vagus, host string, events chan<- voiceEvent) {
	for {
		cmd := exec.CommandContext(ctx, vagus, "tail", "cochlea.hearing", "thalamus.hush")
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		if err == nil {
			dec := json.NewDecoder(stdout)
			for {
				var e voiceEvent
				if dec.Decode(&e) != nil {
					break
				}
				if e.Source.Host != host {
					continue
				}
				select {
				case events <- e:
				case <-ctx.Done():
				}
			}
			err = cmd.Wait()
		}
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "ewwd: vagus tail: %v\n", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(voiceRetry):
		}
	}
}
