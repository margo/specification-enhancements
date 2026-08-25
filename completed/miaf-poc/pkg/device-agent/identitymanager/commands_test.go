package identitymanager

import "testing"

func TestCmd_ZeroValueIsBenign(t *testing.T) {
	var c Cmd
	if c.Renew != nil || c.Refresh != nil || c.Check != nil ||
		c.JWT != nil || c.Status != nil || c.Stop != nil {
		t.Errorf("zero Cmd should have no command set: %+v", c)
	}
}
