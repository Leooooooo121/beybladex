package atc

import (
	"testing"
)

func TestDefaultMMToyShopSession(t *testing.T) {
	if DefaultMMToyShopSession == "" {
		t.Fatalf("DefaultMMToyShopSession 不可為空")
	}
}
