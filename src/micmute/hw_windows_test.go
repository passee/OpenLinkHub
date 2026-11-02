package micmute

import (
	"os"
	"testing"
)

func TestReadEndpoint(t *testing.T) {
	if os.Getenv("OLH_HW_TEST") == "" {
		t.Skip("set OLH_HW_TEST=1")
	}
	muted, err := Muted("HS80")
	t.Logf("muted=%v err=%v", muted, err)
	vol, err := Volume("HS80")
	t.Logf("volume=%d err=%v", vol, err)
}
