package logdistributor

import (
	"testing"
	"fmt"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestPubsub_Subscribe(t *testing.T) {
	ps := New()
	rlsName := "testrls"

	ps.Subscribe(rlsName, rspb.Log_WARNING, rspb.Log_HOOK)
	if len(ps.releases[rlsName].sourceMappings) != 1 {
		t.Error("testrls should have one log source entry")
	}
	ps.Subscribe(rlsName, rspb.Log_WARNING, rspb.Log_POD)
	if len(ps.releases[rlsName].sourceMappings) != 2 {
		t.Error("testrls should have two log source entries")
	}

	if len(ps.releases[rlsName].sourceMappings[rspb.Log_HOOK]) != 1 {
		t.Error("testrls should have one subscription to the Log_HOOK event stream")
	}
	if len(ps.releases[rlsName].sourceMappings[rspb.Log_POD]) != 1 {
		t.Error("testrls should have one subscription to the Log_POD event stream")
	}
	if len(ps.releases[rlsName].sourceMappings[rspb.Log_SYSTEM]) != 0 {
		t.Error("testrls should have no subscriptions to the Log_SYSTEM event stream")
	}
}

func TestPubsub_Unsubscribe(t *testing.T) {
	ps := New()
	rlsName := "testrls"

	sub := ps.Subscribe(rlsName, rspb.Log_WARNING, rspb.Log_HOOK)
	if len(ps.releases[rlsName].sourceMappings) != 1 {
		t.Error("testrls should have one log source entry")
	}

	ps.Unsubscribe(sub)
	if _, ok := ps.releases[rlsName]; ok {
		t.Error("pubsub should have no entry for testrls")
	}

	sub2 := ps.Subscribe(rlsName, rspb.Log_WARNING, rspb.Log_HOOK)
	sub3 := ps.Subscribe(rlsName, rspb.Log_WARNING, rspb.Log_POD)

	if len(ps.releases[rlsName].sourceMappings) != 2 {
		t.Error("testrls should have two log source entries")
	}

	ps.Unsubscribe(sub3)

	if len(ps.releases[rlsName].sourceMappings) != 1 {
		t.Error("testrls should have one log source entry")
	}

	ps.Unsubscribe(sub2)
	if _, ok := ps.releases[rlsName]; ok {
		t.Error("pubsub should have no entry for testrls")
	}

}

func TestPubsub_PubLog(t *testing.T) {

}

func Example() {
	ps := New()
	sub := ps.Subscribe("testrls", rspb.Log_WARNING, rspb.Log_HOOK, rspb.Log_POD)

	go func() {
		for {
			select {
			case l := <-sub.C:
				fmt.Println(l.Log)
			}
		}
	}()

	// No output as log level is too low for the configured subscription
	ps.PubLog("testrls", rspb.Log_POD, rspb.Log_DEBUG, "Test log!")
	// Picked up by the subscription
	ps.PubLog("testrls", rspb.Log_POD, rspb.Log_ERR, "Test log!")
	// Output: Test log!
}
