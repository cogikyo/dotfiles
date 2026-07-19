package providers

import "testing"

func TestParseAudioIdentity(t *testing.T) {
	output := `id 64, type PipeWire:Interface:Node
  * node.description = "Scarlett Solo (3rd Gen.) Input 1 Mic"
  * node.name = "alsa_input.usb-Focusrite"
  * node.nick = "Scarlett Solo USB"
`
	stable, display, ok := parseAudioIdentity(output)
	if !ok {
		t.Fatal("parseAudioIdentity reported unavailable")
	}
	if stable != "alsa_input.usb-Focusrite" {
		t.Fatalf("stable name = %q", stable)
	}
	if display != "Scarlett Solo (3rd Gen.) Input 1 Mic" {
		t.Fatalf("display name = %q", display)
	}
}

func TestParseAudioVolume(t *testing.T) {
	tests := []struct {
		name                string
		output              string
		wantPercent         int
		wantMuted, wantOkay bool
	}{
		{"amplified muted", "Volume: 1.30 [MUTED]\n", 130, true, true},
		{"malformed", "Volume: nope", 0, false, false},
		{"NaN", "Volume: NaN", 0, false, false},
		{"+Inf", "Volume: +Inf", 0, false, false},
		{"-Inf", "Volume: -Inf", 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			percent, muted, ok := parseAudioVolume(tt.output)
			if percent != tt.wantPercent || muted != tt.wantMuted || ok != tt.wantOkay {
				t.Fatalf("parseAudioVolume = (%d, %t, %t), want (%d, %t, %t)", percent, muted, ok, tt.wantPercent, tt.wantMuted, tt.wantOkay)
			}
		})
	}
}

func TestAudioEventConfirmsCombinedReset(t *testing.T) {
	events := map[string]bool{"sink": true}
	if audioEventConfirms("both", events) {
		t.Fatal("sink event alone confirmed combined reset")
	}
	events["source"] = true
	if !audioEventConfirms("both", events) {
		t.Fatal("sink and source events did not confirm combined reset")
	}
}
