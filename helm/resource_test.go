package helm

import (
	"fmt"
	"testing"
)

func TestKeyMutex(t *testing.T) {
	t.Parallel()

	chartFile := "harbor-helm-v.19.1.tar.gz"
	images, err := Images(chartFile)
	if err != nil {
		t.Errorf("Run test error: %v", err)
	}
	for _, i := range images {
		fmt.Println(i)
	}
}
