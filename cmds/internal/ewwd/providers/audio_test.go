package providers

import (
	"context"
	"testing"
	"time"

	"dotfiles/cmds/internal/config"
)

type volumeWrite struct {
	node    string
	percent int
}

func testAudio(t *testing.T) (*Audio, <-chan volumeWrite, chan<- struct{}) {
	writes := make(chan volumeWrite, 16)
	release := make(chan struct{})
	a := NewAudio(&stateMap{data: map[string]any{}}, config.AudioConfig{SinkMax: 100, SourceMax: 100, DefaultSinkVolume: 69, DefaultSourceVolume: 50, VolumeStep: 1}, "")
	a.write = func(ctx context.Context, node string, percent, ceiling int) error {
		writes <- volumeWrite{node: node, percent: percent}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
			t.Error("write never released")
		}
		return nil
	}
	a.publish(AudioState{SinkAvailable: true, Sink: 50, SourceAvailable: true, Source: 50})
	a.sink = audioDevice{id: "10", volume: 50}
	return a, writes, release
}

func request(t *testing.T, a *Audio, args ...string) {
	t.Helper()
	changes, ok, err := a.volumeRequest(args)
	if !ok || err != nil {
		t.Fatalf("volumeRequest(%v) = ok %t, err %v", args, ok, err)
	}
	a.requestVolume(context.Background(), changes)
}

func finish(t *testing.T, a *Audio, release chan<- struct{}) bool {
	t.Helper()
	release <- struct{}{}
	select {
	case done := <-a.applied:
		return a.volumeDone(context.Background(), done)
	case <-time.After(time.Second):
		t.Fatal("no write completion")
		return false
	}
}

func TestAudioResetFollowsQueuedVolume(t *testing.T) {
	a, writes, release := testAudio(t)

	request(t, a, "set_volume", "sink", "60")
	if got := <-writes; got != (volumeWrite{"10", 60}) {
		t.Fatalf("first write = %+v", got)
	}
	request(t, a, "set_volume", "sink", "61")
	request(t, a, "reset_volume", "sink")

	if !finish(t, a, release) {
		t.Fatal("queued reset was not written after the first write finished")
	}
	if got := <-writes; got != (volumeWrite{"10", 69}) {
		t.Fatalf("second write = %+v, want the reset to 69", got)
	}
	if finish(t, a, release) {
		t.Fatal("a write followed the reset")
	}
	select {
	case got := <-writes:
		t.Fatalf("write %+v landed after the reset", got)
	case <-time.After(50 * time.Millisecond):
	}
	if a.last.Sink != 69 {
		t.Fatalf("published sink = %d, want 69", a.last.Sink)
	}
}

func TestAudioDeviceSwitchDropsQueuedVolume(t *testing.T) {
	a, writes, release := testAudio(t)

	request(t, a, "set_volume", "sink", "60")
	<-writes
	request(t, a, "set_volume", "sink", "61")

	if got := a.pendingVolume("sink", audioDevice{id: "20", volume: 30}); got != 30 {
		t.Fatalf("new default shows %d, want its own volume 30", got)
	}
	if finish(t, a, release) {
		t.Fatal("queued volume was written after the default device changed")
	}
	select {
	case got := <-writes:
		t.Fatalf("unexpected write %+v after the device switch", got)
	case <-time.After(50 * time.Millisecond):
	}
}
