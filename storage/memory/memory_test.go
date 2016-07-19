package memory

import (
	"testing"

	"github.com/ericchiang/poke/storage/storagetest"
)

func TestStorage(t *testing.T) {
	s := New()
	storagetest.RunTestSuite(t, s)
}
