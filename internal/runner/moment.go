package runner

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// Moments: with the global claim moments setting on, a watch (drop or force)
// on a backend that is a platform.MomentClaimer listens for the moments its
// channel hands out. A moment announced there is claimed once per moment
// ID, after a short random delay, only if the same watch is still live.
// Like raids, claiming is a side task on a watch-scoped context: it never
// touches the beacon, progress or claims, and the watch keeps its channel.

// momentSeenFor is how long a moment ID stays in the dedupe set. A moment
// stays claimable for a few minutes after it is announced.
const momentSeenFor = time.Hour

var (
	// momentDelay is the pause before claiming a moment, random in
	// [2s, 10s). A var so tests can shrink it.
	momentDelay = func() time.Duration {
		return 2*time.Second + rand.N(8*time.Second)
	}
	// momentTimeout bounds one claim attempt.
	momentTimeout = 10 * time.Second
)

// momentWatch is the moments side task of one watch: the watched channel
// and the context its claim goroutines run under.
type momentWatch struct {
	channelID string
	cancel    context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup // claim goroutines in flight
}

// stop cancels pending claims, waits for their goroutines and drops the
// moments topic. nil-safe.
func (w *momentWatch) stop(r *Runner) {
	if w == nil {
		return
	}
	w.cancel()
	w.wg.Wait()
	if mc, ok := r.cfg.Backend.(platform.MomentClaimer); ok {
		mc.UnsubscribeMoments(r.cfg.AccountID, w.channelID)
	}
}

// claimMoments reads the claim moments setting now.
func (r *Runner) claimMoments() bool {
	return r.cfg.ClaimMoments != nil && r.cfg.ClaimMoments()
}

// momentClaimer is the backend's moment claimer when claim moments is on,
// else ok is false.
func (r *Runner) momentClaimer() (platform.MomentClaimer, bool) {
	if !r.claimMoments() {
		return nil, false
	}
	mc, ok := r.cfg.Backend.(platform.MomentClaimer)
	return mc, ok
}

// startMoments subscribes the live watch's channel to moments, scoped to a
// child of ctx. It replaces any moments task still running. A no-op when
// claim moments is off, the backend cannot claim moments, or no watch is
// live on the channel.
func (r *Runner) startMoments(ctx context.Context, channelID string) {
	mc, ok := r.momentClaimer()
	if !ok || channelID == "" {
		return
	}
	mctx, cancel := context.WithCancel(ctx)
	w := &momentWatch{channelID: channelID, ctx: mctx, cancel: cancel}
	r.mu.Lock()
	if r.handle == nil || r.stream == nil || r.stream.ChannelID != channelID {
		r.mu.Unlock()
		cancel()
		return
	}
	prev := r.moments
	r.moments = w
	r.mu.Unlock()
	prev.stop(r) // before the subscribe, so a same-channel prev cannot drop it
	mc.SubscribeMoments(r.cfg.AccountID, channelID)
}

// stopMoments ends the running moments task, if any.
func (r *Runner) stopMoments() {
	r.mu.Lock()
	w := r.moments
	r.moments = nil
	r.mu.Unlock()
	w.stop(r)
}

// syncMoments starts or stops the live watch's moments task to match the
// claim moments setting now, keeping the watch. Run goroutine only.
func (r *Runner) syncMoments(ctx context.Context) {
	_, on := r.momentClaimer()
	r.mu.Lock()
	running := r.moments != nil
	channelID := ""
	if r.handle != nil && r.stream != nil {
		channelID = r.stream.ChannelID
	}
	r.mu.Unlock()
	switch {
	case on && !running && channelID != "":
		r.startMoments(ctx, channelID)
	case !on && running:
		r.stopMoments()
	}
}

// onMoment schedules one claim of momentID when it comes from the channel
// the account is watching with claim moments on. A repeat of a moment ID
// already scheduled is ignored.
func (r *Runner) onMoment(channelID, momentID string) {
	if momentID == "" {
		return
	}
	mc, ok := r.momentClaimer()
	if !ok {
		return
	}
	now := time.Now()
	r.mu.Lock()
	w := r.moments
	if w == nil || w.channelID != channelID || w.ctx.Err() != nil {
		r.mu.Unlock()
		return
	}
	if _, dup := r.momentSeen[momentID]; dup {
		r.mu.Unlock()
		return
	}
	if r.momentSeen == nil {
		r.momentSeen = map[string]time.Time{}
	}
	for id, at := range r.momentSeen {
		if now.Sub(at) > momentSeenFor {
			delete(r.momentSeen, id)
		}
	}
	r.momentSeen[momentID] = now
	// Added under r.mu: stop takes r.moments under it first, so no claim
	// is added after its Wait starts.
	w.wg.Add(1)
	r.mu.Unlock()
	go r.claimMoment(w, mc, momentID, momentDelay())
}

// claimMoment waits delay, then claims momentID if w is still the live
// watch's moments task on its channel and claim moments is still on.
func (r *Runner) claimMoment(w *momentWatch, mc platform.MomentClaimer, momentID string, delay time.Duration) {
	defer w.wg.Done()
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-w.ctx.Done():
		return
	case <-t.C:
	}
	r.mu.Lock()
	still := r.moments == w && r.handle != nil && r.stream != nil && r.stream.ChannelID == w.channelID
	channel := ""
	if r.stream != nil {
		channel = r.stream.Channel
	}
	r.mu.Unlock()
	if !still || !r.claimMoments() {
		return
	}
	ctx, cancel := context.WithTimeout(w.ctx, momentTimeout)
	defer cancel()
	if err := mc.ClaimMoment(ctx, r.cfg.Session, momentID); err != nil {
		if w.ctx.Err() == nil {
			slog.Warn("runner claim moment failed", "kind", "error", "account", r.cfg.AccountID,
				"channel", channel, "moment", momentID, "err", err)
		}
		return
	}
	slog.Info("runner claimed moment", "kind", "info", "account", r.cfg.AccountID,
		"channel", channel, "moment", momentID)
}
