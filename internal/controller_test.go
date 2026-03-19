package internal

import (
	"testing"
	"time"
)

var noop Logger = func(string, ...any) {}

func TestController_AddOrder_QueuePriority(t *testing.T) {
	c := newControllerWithDuration(noop, time.Hour) // no bots → orders stay in queue

	c.AddOrder(Normal) // #1
	c.AddOrder(VIP)    // #2 → goes before Normal
	c.AddOrder(Normal) // #3
	c.AddOrder(VIP)    // #4 → goes behind VIP#2 but before Normals

	c.mu.Lock()
	defer c.mu.Unlock()

	assertOrder(t, c.queue.Pop(), 2, VIP)
	assertOrder(t, c.queue.Pop(), 4, VIP)
	assertOrder(t, c.queue.Pop(), 1, Normal)
	assertOrder(t, c.queue.Pop(), 3, Normal)
}

func TestController_BotProcessesOrder(t *testing.T) {
	c := newControllerWithDuration(noop, 50*time.Millisecond)

	c.AddOrder(Normal) // #1
	c.AddBot()         // picks up #1

	time.Sleep(150 * time.Millisecond)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.queue.Len() != 0 {
		t.Errorf("expected empty queue, got %d pending", c.queue.Len())
	}
	if len(c.bots) != 1 || c.bots[0].Status != Idle {
		t.Error("expected bot to be IDLE after processing")
	}
}

func TestController_IdleBotPicksUpNewOrder(t *testing.T) {
	c := newControllerWithDuration(noop, time.Hour)

	c.AddBot() // IDLE — no orders yet

	c.mu.Lock()
	if c.bots[0].Status != Idle {
		t.Error("expected bot to be IDLE")
	}
	c.mu.Unlock()

	c.AddOrder(Normal) // IDLE bot should pick this up immediately

	time.Sleep(20 * time.Millisecond)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.bots[0].Status != Busy {
		t.Error("expected bot to be PROCESSING after new order arrived")
	}
}

func TestController_RemoveBot_ReturnsOrderToPending(t *testing.T) {
	c := newControllerWithDuration(noop, time.Hour)

	c.AddOrder(Normal) // #1
	c.AddBot()         // picks up #1

	time.Sleep(20 * time.Millisecond) // bot is processing

	c.RemoveBot() // order should return to PENDING

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.queue.Len() != 1 {
		t.Errorf("expected 1 pending order after bot removal, got %d", c.queue.Len())
	}
	if len(c.bots) != 0 {
		t.Error("expected no bots after removal")
	}
}

func TestController_RemoveBot_NewestFirst(t *testing.T) {
	c := newControllerWithDuration(noop, time.Hour)

	c.AddOrder(Normal) // #1
	c.AddOrder(Normal) // #2
	bot1 := c.AddBot() // picks up #1
	bot2 := c.AddBot() // picks up #2

	time.Sleep(20 * time.Millisecond)

	c.RemoveBot() // removes bot2 (highest ID)

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.bots) != 1 || c.bots[0].ID != bot1.ID {
		t.Errorf("expected bot#%d to remain, bots: %v", bot1.ID, c.bots)
	}
	_ = bot2
}
