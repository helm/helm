package logdistributor

import (
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

type Logsub struct {
	C       chan *rspb.Log
	release string
	sources []rspb.LogSource
	level   rspb.LogLevel
}

type release struct {
	name           string
	sourceMappings map[rspb.LogSource]map[*Logsub]bool
}

type Pubsub struct {
	releases map[string]*release
}

func New() *Pubsub {
	rls := make(map[string]*release)
	return &Pubsub{releases: rls}
}

func newRelease(name string) *release {
	rs := &release{name: name}
	rs.sourceMappings = make(map[rspb.LogSource]map[*Logsub]bool, len(rspb.LogSource_name))
	return rs
}

func (rs *release) subscribe(sub *Logsub) {
	for _, source := range sub.sources {
		logSource := rspb.LogSource(source)
		if _, ok := rs.sourceMappings[logSource]; !ok {
			subs := make(map[*Logsub]bool, 1)
			rs.sourceMappings[logSource] = subs
		}
		rs.sourceMappings[logSource][sub] = true
	}
}

func (ps *Pubsub) subscribe(sub *Logsub) {
	if _, ok := ps.releases[sub.release]; !ok {
		rs := newRelease(sub.release)
		rs.subscribe(sub)
		ps.releases[sub.release] = rs
	}
	ps.releases[sub.release].subscribe(sub)
}

func (ps *Pubsub) Subscribe(release string, level rspb.LogLevel, sources ...rspb.LogSource) *Logsub {
	ch := make(chan *rspb.Log)
	ls := &Logsub{C: ch, release: release, level: level, sources: sources}
	ps.subscribe(ls)
	return ls
}

func (ps *Pubsub) Unsubscribe(sub *Logsub) {
	if rs, ok := ps.releases[sub.release]; ok {
		for source, subMap := range rs.sourceMappings {
			delete(subMap, sub)
			if len(subMap) == 0 {
				delete(rs.sourceMappings, source)
			}
		}
		if len(rs.sourceMappings) == 0 {
			delete(ps.releases, sub.release)
		}
	}
}

func (ps *Pubsub) PubLog(rls string, source rspb.LogSource, level rspb.LogLevel, message string) {
	log := &rspb.Log{Release: rls, Source: source, Level: level, Log: message}
	if rls, ok := ps.releases[log.Release]; ok {
		if subs, ok := rls.sourceMappings[log.Source]; ok {
			for sub := range subs {
				if sub.level >= log.Level {
					sub.C <- log
				}
			}
		}
	}
}

