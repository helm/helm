package logdistributor

import "fmt"

type Log struct {
	Log string
}

type Subscription struct {
	c chan<- *Log
}

type Listener struct {
	subs map[*Subscription]bool
}

type Distributor struct {
	listeners map[string]*Listener
}

func (l *Listener) subscribe(c chan<- *Log) *Subscription {
	sub := &Subscription{c}
	l.subs[sub] = true
	return sub
}

func (d *Distributor) Subscribe() {

}

func (l *Listener) unsubscribe(sub *Subscription) {
	delete(l.subs, sub)
}

func (l *Listener) writeLog(log *Log) error {
	for _, s := range l.subs {
		s.c <- log
	}
	return nil
}

func (d *Distributor) WriteLog(log *Log, release string) error {
	l := d.listeners[release]
	if l == nil {
		return fmt.Errorf("No listeners configured for %s", release)
	}
	return l.writeLog(log)
}
