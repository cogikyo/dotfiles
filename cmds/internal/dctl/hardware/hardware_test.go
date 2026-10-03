package hardware

import (
	"slices"
	"testing"
)

func TestPending(t *testing.T) {
	got, err := pending([]byte(`{"Devices":[{"Name":"BIOS","Releases":[{"Version":"3.07"}]},{"Name":"SSD","Releases":[]}]}`), nil)
	if err != nil || !slices.Equal(got, []string{"BIOS 3.07"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}
