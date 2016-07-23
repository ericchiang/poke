package kubernetes

import (
	"testing"

	"github.com/ericchiang/poke/storage/kubernetes/types/extensions/v1beta1"
)

func TestStorage(t *testing.T) {
	client := loadClient(t)
	var thirdPartyResources v1beta1.ThirdPartyResourceList
	if err := client.list("extensions", "v1beta1", "", "thirdpartyresources", &thirdPartyResources); err != nil {
		t.Fatal(err)
	}
	t.Log(thirdPartyResources)
}
