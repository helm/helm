package logs

import (
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"strings"
)

type Logsub struct {
	C       chan *rspb.Log
	release string
	sources []rspb.Log_Source
	level   rspb.Log_Level
}

type LogWriter struct {
	rls string
	source rspb.Log_Source
	level rspb.Log_Level
	ps *Pubsub
}

type release struct {
	name           string
	sourceMappings map[rspb.Log_Source]map[*Logsub]bool
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
	rs.sourceMappings = make(map[rspb.Log_Source]map[*Logsub]bool, len(rspb.Log_Source_name))
	return rs
}

func (rs *release) subscribe(sub *Logsub) {
	for _, source := range sub.sources {
		Log_Source := rspb.Log_Source(source)
		if _, ok := rs.sourceMappings[Log_Source]; !ok {
			subs := make(map[*Logsub]bool, 1)
			rs.sourceMappings[Log_Source] = subs
		}
		rs.sourceMappings[Log_Source][sub] = true
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

func (ps *Pubsub) Subscribe(release string, level rspb.Log_Level, sources ...rspb.Log_Source) *Logsub {
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

func (ps *Pubsub) PubLog(rls string, source rspb.Log_Source, level rspb.Log_Level, message string) {
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

func (ps *Pubsub) GetWriter(rls string, source rspb.Log_Source, level rspb.Log_Level) *LogWriter {
	return &LogWriter{rls: rls, source: source, level: level, ps: ps}
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	logs := strings.Split(string(p), "\n")
	for _, l := range logs {
		lw.ps.PubLog(lw.rls, lw.source, lw.level, l)
	}
	return len(p), nil
}
