package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type progressBar struct {
	label       string
	total       int64
	startedAt   time.Time
	finishedAt  time.Time
	uploaded    int64
	lastPercent int
	lastRender  time.Time
}

func newProgressBar(label string, total int64) *progressBar {
	return &progressBar{
		label:     label,
		total:     total,
		startedAt: time.Now(),
	}
}

func (p *progressBar) reader(r io.Reader) io.Reader {
	return &progressReader{r: r, p: p}
}

func (p *progressBar) add(n int) {
	if n <= 0 {
		return
	}
	p.uploaded += int64(n)

	percent := 0
	if p.total > 0 {
		percent = int(p.uploaded * 100 / p.total)
	}
	now := time.Now()
	if p.uploaded >= p.total && p.total > 0 && p.finishedAt.IsZero() {
		p.finishedAt = now
	}
	if percent != p.lastPercent || now.Sub(p.lastRender) >= 150*time.Millisecond || p.uploaded >= p.total {
		p.lastPercent = percent
		p.lastRender = now
		p.render(false)
	}
}

func (p *progressBar) finish() {
	if p.total > 0 {
		p.uploaded = p.total
		p.lastPercent = 100
	}
	if p.finishedAt.IsZero() {
		p.finishedAt = time.Now()
	}
	p.render(true)
	fmt.Println()
}

func (p *progressBar) render(done bool) {
	const width = 24

	filled := 0
	if p.total > 0 {
		filled = int(p.uploaded * width / p.total)
	}
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("=", filled)
	if done {
		bar = strings.Repeat("=", width)
	} else if filled < width {
		bar += ">"
		bar += strings.Repeat(" ", width-filled-1)
	}

	speed := p.speed()
	fmt.Printf("\r\033[2K%s [%s] %3d%% %s/%s %s", p.label, bar, p.lastPercent, formatBytes(float64(p.uploaded)), formatBytes(float64(p.total)), speed)
}

func (p *progressBar) speed() string {
	end := time.Now()
	if !p.finishedAt.IsZero() {
		end = p.finishedAt
	}
	elapsed := end.Sub(p.startedAt).Seconds()
	if elapsed <= 0 {
		return "0B/s"
	}
	return formatBytes(float64(p.uploaded)/elapsed) + "/s"
}

func formatBytes(v float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	unit := 1024.0
	i := 0
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f%s", v, units[i])
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}

type progressReader struct {
	r io.Reader
	p *progressBar
}

func (pr *progressReader) Read(buf []byte) (int, error) {
	n, err := pr.r.Read(buf)
	pr.p.add(n)
	return n, err
}
