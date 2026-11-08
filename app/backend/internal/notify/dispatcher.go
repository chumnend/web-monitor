package notify

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

// Dispatcher processes site state transitions, handles debounce logic, and fans out to Senders.
type Dispatcher struct {
	store          store.Store
	senders        map[store.ChannelType]Sender
	debounceWindow time.Duration
}

// NewDispatcher creates a new notification Dispatcher.
func NewDispatcher(s store.Store, window ...time.Duration) *Dispatcher {
	win := 5 * time.Minute
	if len(window) > 0 {
		win = window[0]
	}

	return &Dispatcher{
		store:          s,
		senders:        make(map[store.ChannelType]Sender),
		debounceWindow: win,
	}
}

// RegisterSender registers a concrete Sender implementation for a channel type.
func (d *Dispatcher) RegisterSender(cType store.ChannelType, sender Sender) {
	d.senders[cType] = sender
}

// HandleTransition processes a state transition. It is designed to be attached as a checker.TransitionListener.
func (d *Dispatcher) HandleTransition(site *store.Site, transition store.NotificationTransition, prevStatus store.SiteStatus) {
	// Provide a bounded context since this usually runs in a background goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	channels, err := d.store.GetChannelsForSiteTransition(ctx, site)
	if err != nil {
		log.Printf("Dispatcher error fetching channels for site %s: %v", site.ID, err)
		return
	}

	for _, ch := range channels {
		d.processChannel(ctx, site, ch, transition)
	}
}

func (d *Dispatcher) processChannel(ctx context.Context, site *store.Site, ch *store.NotificationChannel, transition store.NotificationTransition) {
	// 1. Check Debounce
	lastEvent, err := d.store.GetLastNotificationEvent(ctx, site.ID, ch.ID)
	if err != nil {
		log.Printf("Dispatcher error fetching last event for channel %s: %v", ch.ID, err)
		return
	}

	if lastEvent != nil {
		timeSinceLast := time.Since(lastEvent.SentAt)
		if timeSinceLast < d.debounceWindow {
			// Suppress due to debounce storm protection
			status := store.DeliveryStatusSuppressedDebounce
			_ = d.store.RecordNotificationEvent(ctx, &store.NotificationEvent{
				ID:             uuid.New(),
				SiteID:         site.ID,
				ChannelID:      ch.ID,
				Transition:     transition,
				SentAt:         time.Now().UTC(),
				DeliveryStatus: &status,
			})
			return
		}
	}

	// 2. Dispatch
	sender, ok := d.senders[ch.Type]
	if !ok {
		log.Printf("Dispatcher missing sender for channel type %s", ch.Type)
		status := store.DeliveryStatusFailed
		_ = d.store.RecordNotificationEvent(ctx, &store.NotificationEvent{
			ID:             uuid.New(),
			SiteID:         site.ID,
			ChannelID:      ch.ID,
			Transition:     transition,
			SentAt:         time.Now().UTC(),
			DeliveryStatus: &status,
		})
		return
	}

	name := site.NormalizedURL
	if site.Label != nil {
		name = *site.Label
	}

	msg := Message{
		SiteName:   name,
		SiteURL:    site.URL,
		Transition: transition,
		Timestamp:  time.Now().UTC(),
	}

	status := store.DeliveryStatusSent
	if err := sender.Send(ctx, ch.Target, msg); err != nil {
		log.Printf("Dispatcher send failed for channel %s: %v", ch.ID, err)
		status = store.DeliveryStatusFailed
	}

	// 3. Record Outcome
	_ = d.store.RecordNotificationEvent(ctx, &store.NotificationEvent{
		ID:             uuid.New(),
		SiteID:         site.ID,
		ChannelID:      ch.ID,
		Transition:     transition,
		SentAt:         time.Now().UTC(),
		DeliveryStatus: &status,
	})
}
