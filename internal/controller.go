package internal

import (
	"context"
	"sync"
	"time"
)

// Logger is a function that records a timestamped line of output.
type Logger func(format string, args ...any)

// Controller manages the order queue and bots.
type Controller struct {
	mu              sync.Mutex
	queue           *Queue
	bots            []*Bot
	nextOrderID     int
	nextBotID       int
	completedOrders []*Order
	log             Logger
	processingTime  time.Duration
}

func NewController(log Logger) *Controller {
	return &Controller{
		queue:          &Queue{},
		log:            log,
		processingTime: 10 * time.Second,
	}
}

// newControllerWithDuration is used in tests to speed up processing time.
func newControllerWithDuration(log Logger, d time.Duration) *Controller {
	return &Controller{
		queue:          &Queue{},
		log:            log,
		processingTime: d,
	}
}

// AddOrder creates a new order, places it in the queue, and assigns it to an
// idle bot if one is available.
func (c *Controller) AddOrder(t OrderType) *Order {
	c.mu.Lock()
	c.nextOrderID++
	order := &Order{ID: c.nextOrderID, Type: t, Status: Pending}
	c.queue.Push(order)
	c.log("New %s Order #%d → PENDING", t, order.ID)

	var idleBot *Bot
	for _, b := range c.bots {
		if b.Status == Idle {
			idleBot = b
			break
		}
	}
	c.mu.Unlock()

	if idleBot != nil {
		c.pickUp(idleBot)
	}
	return order
}

// AddBot creates a new bot and immediately assigns it to a pending order if any.
func (c *Controller) AddBot() *Bot {
	c.mu.Lock()
	c.nextBotID++
	bot := &Bot{ID: c.nextBotID, Status: Idle}
	c.bots = append(c.bots, bot)
	c.log("Bot #%d created → IDLE", bot.ID)
	c.mu.Unlock()

	c.pickUp(bot)
	return bot
}

// RemoveBot destroys the newest bot (highest ID). If it is processing an order,
// the order is returned to the queue in its original priority position.
func (c *Controller) RemoveBot() {
	c.mu.Lock()
	if len(c.bots) == 0 {
		c.mu.Unlock()
		return
	}

	bot := c.bots[len(c.bots)-1]
	c.bots = c.bots[:len(c.bots)-1]

	if bot.Status == Busy && bot.CurrentOrder != nil {
		order := bot.CurrentOrder
		bot.CurrentOrder = nil // clear before signalling goroutine
		bot.cancel()
		order.Status = Pending
		c.queue.PushByID(order)
		c.log("Bot #%d removed → Order #%d returned to PENDING", bot.ID, order.ID)
	} else {
		c.log("Bot #%d removed → was IDLE", bot.ID)
	}
	c.mu.Unlock()
}

// Status logs the current state of bots and pending orders.
func (c *Controller) Status() {
	c.mu.Lock()
	defer c.mu.Unlock()

	processing := 0
	for _, b := range c.bots {
		if b.Status == Busy {
			processing++
		}
	}
	c.log("Status: bots=%d (processing=%d, idle=%d), pending=%d",
		len(c.bots), processing, len(c.bots)-processing, c.queue.Len())
}

// FinalStatus logs a summary of all completed orders and remaining state.
func (c *Controller) FinalStatus() {
	c.mu.Lock()
	defer c.mu.Unlock()

	vipDone, normalDone := 0, 0
	for _, o := range c.completedOrders {
		if o.Type == VIP {
			vipDone++
		} else {
			normalDone++
		}
	}

	c.log("=== Final Status ===")
	c.log("Total Orders Completed: %d (VIP: %d, Normal: %d)", len(c.completedOrders), vipDone, normalDone)
	c.log("Active Bots: %d", len(c.bots))
	c.log("Pending Orders: %d", c.queue.Len())
}

// pickUp assigns the next pending order to bot, or marks bot IDLE.
func (c *Controller) pickUp(bot *Bot) {
	c.mu.Lock()

	// Ensure bot is still registered (may have been removed concurrently).
	found := false
	for _, b := range c.bots {
		if b.ID == bot.ID {
			found = true
			break
		}
	}
	if !found || bot.CurrentOrder != nil {
		c.mu.Unlock()
		return
	}

	order := c.queue.Pop()
	if order == nil {
		bot.Status = Idle
		c.log("Bot #%d → IDLE (no pending orders)", bot.ID)
		c.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	bot.Status = Busy
	bot.CurrentOrder = order
	bot.cancel = cancel
	order.Status = Processing
	c.log("Bot #%d picks up %s Order #%d → PROCESSING", bot.ID, order.Type, order.ID)
	c.mu.Unlock()

	go c.process(bot, order, ctx)
}

// process waits processingTime, then completes the order and picks the next one.
// If the bot is removed mid-processing, ctx is cancelled and the goroutine exits.
func (c *Controller) process(bot *Bot, order *Order, ctx context.Context) {
	select {
	case <-time.After(c.processingTime):
		c.mu.Lock()
		// If bot.CurrentOrder was cleared by RemoveBot, abort — order already re-queued.
		if bot.CurrentOrder != order {
			c.mu.Unlock()
			return
		}
		order.Status = Complete
		bot.CurrentOrder = nil
		c.completedOrders = append(c.completedOrders, order)
		c.log("Bot #%d completed %s Order #%d → COMPLETE", bot.ID, order.Type, order.ID)
		c.mu.Unlock()

		c.pickUp(bot)

	case <-ctx.Done():
		// Bot was removed; RemoveBot already re-queued the order.
		return
	}
}
