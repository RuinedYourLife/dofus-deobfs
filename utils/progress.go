package utils

import "sync/atomic"

// Global progress tracking
type MatchingProgress struct {
	totalMessages int64
	matchedSoFar  int64
}

var GlobalProgress = &MatchingProgress{}

func (p *MatchingProgress) Init(total int) {
	atomic.StoreInt64(&p.totalMessages, int64(total))
	atomic.StoreInt64(&p.matchedSoFar, 0)
}

func (p *MatchingProgress) AddMatches(count int) {
	atomic.AddInt64(&p.matchedSoFar, int64(count))
}

func (p *MatchingProgress) GetProgress() float64 {
	total := atomic.LoadInt64(&p.totalMessages)
	if total == 0 {
		return 0
	}
	matched := atomic.LoadInt64(&p.matchedSoFar)
	return float64(matched) / float64(total) * 100
}
