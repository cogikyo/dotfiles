package providers

import (
	"strings"
	"testing"
)

type musicTestState struct{}

func (musicTestState) Set(string, any) {}

func TestParseFollowStateConvertsMPRISMicroseconds(t *testing.T) {
	state, err := parseFollowState(strings.Join([]string{"Playing", "0.65", "Artist", "Album", "Title", "art", "track", "245000000"}, followSeparator))
	if err != nil {
		t.Fatal(err)
	}
	if state.duration != 245 {
		t.Fatalf("duration = %v, want 245", state.duration)
	}
}

func TestParseFollowStatePreservesTabsInMetadata(t *testing.T) {
	state, err := parseFollowState(strings.Join([]string{"Playing", "0.65", "Artist\tfeat. Guest", "Album\tDeluxe", "Title\tLive", "art", "track", "245000000"}, followSeparator))
	if err != nil {
		t.Fatal(err)
	}
	if state.artist != "Artist\tfeat. Guest" || state.album != "Album\tDeluxe" || state.title != "Title\tLive" {
		t.Fatalf("metadata lost tabs: %#v", state)
	}
}

func TestPositionRevisionGuardsDiscardObsoleteReads(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*musicOwner)
	}{
		{
			name: "track",
			mutate: func(owner *musicOwner) {
				owner.track = "track-b"
				owner.trackRevision++
			},
		},
		{
			name:   "seek",
			mutate: func(owner *musicOwner) { owner.seekRevision++ },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			owner := musicOwner{
				music:         &Music{state: musicTestState{}},
				track:         "track-a",
				trackRevision: 2,
				seekRevision:  3,
				last:          musicState("Playing", "0.5", "artist", "album", "title", 40),
			}
			stale := owner.positionTicket("poll")
			test.mutate(&owner)
			if owner.applyPosition(stale, 12) {
				t.Fatal("obsolete position read was accepted")
			}
			if owner.last.Progress != 40 {
				t.Fatalf("progress = %d, want 40", owner.last.Progress)
			}
		})
	}

	owner := musicOwner{
		music:         &Music{state: musicTestState{}},
		track:         "track-a",
		trackRevision: 2,
		seekRevision:  3,
		last:          musicState("Playing", "0.5", "artist", "album", "title", 40),
	}
	if !owner.applyPosition(owner.positionTicket("seek"), 60) {
		t.Fatal("current position read was discarded")
	}
	if owner.last.Progress != 60 {
		t.Fatalf("progress = %d, want 60", owner.last.Progress)
	}
}

func TestSeekBlocksPollingUntilTargetedReadFinishes(t *testing.T) {
	owner := musicOwner{
		music:         &Music{state: musicTestState{}},
		track:         "track-a",
		trackRevision: 2,
		seekRevision:  3,
		seekPending:   true,
		last:          musicState("Playing", "0.5", "artist", "album", "title", 40),
	}

	owner.poll(nil)
	if owner.pollPending {
		t.Fatal("poll started during seek")
	}

	seek := owner.positionTicket("seek")
	if !owner.applyPosition(seek, 60) {
		t.Fatal("current seek position read was discarded")
	}
	if owner.seekPending {
		t.Fatal("seek remained pending after successful read")
	}

	owner.seekPending = true
	owner.positionFailed(owner.positionTicket("seek"))
	if owner.seekPending {
		t.Fatal("seek remained pending after failed read")
	}
}
