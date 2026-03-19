package internal

import "context"

type BotStatus string

const (
	Idle BotStatus = "IDLE"
	Busy BotStatus = "PROCESSING"
)

type Bot struct {
	ID           int
	Status       BotStatus
	CurrentOrder *Order
	cancel       context.CancelFunc
}
