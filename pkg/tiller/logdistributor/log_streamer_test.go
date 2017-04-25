package logdistributor

import (
	"testing"
	"fmt"
)

func TestDistributor_WriteLog(t *testing.T) {
	d := &Distributor{}
	l := &Log{Log: "Test log"}
	d.WriteLog(l, "testrelease")

	if len(d.listeners) != 1 {
		t.Errorf("Invalid number of listeners present: %d (expecting 1)", len(d.listeners))
	}
}

func BenchmarkDistributor_WriteLog(b *testing.B) {
}

func ExampleDistributor_WriteLog() {
	sub := &Subscription{}
	c := make(chan *Log)
	sub.c = c

	go func(){
		for l := range c {
			fmt.Println(l.Log)
		}

		for {
			select {
			case l := <-c:
				fmt.Println(l.Log)
			}
		}
	}()

	sub.c <- &Log{Log: "Test log!"}
	// Output: Test log!
}

