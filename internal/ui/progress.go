package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type Progress struct {
	writer  io.Writer
	label   string
	start   time.Time
	enabled bool
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

func StartProgress(writer io.Writer, label string, enabled bool) *Progress {
	p := &Progress{writer: writer, label: label, start: time.Now(), enabled: enabled}
	if !enabled {
		return p
	}
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	go p.run()
	return p
}

func (p *Progress) run() {
	defer close(p.done)
	frames := []byte{'|', '/', '-', '\\'}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	p.render(frames[i])
	for {
		select {
		case <-ticker.C:
			i = (i + 1) % len(frames)
			p.render(frames[i])
		case <-p.stop:
			return
		}
	}
}

func (p *Progress) render(frame byte) {
	fmt.Fprintf(p.writer, "\r%c %s — %s", frame, p.label, time.Since(p.start).Round(time.Second))
}

func (p *Progress) Stop() {
	if !p.enabled {
		return
	}
	p.once.Do(func() {
		close(p.stop)
		<-p.done
		fmt.Fprintf(p.writer, "\r%s\r", strings.Repeat(" ", len(p.label)+32))
	})
}
