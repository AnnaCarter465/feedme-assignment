package internal

type OrderType string
type OrderStatus string

const (
	Normal OrderType = "NORMAL"
	VIP    OrderType = "VIP"

	Pending    OrderStatus = "PENDING"
	Processing OrderStatus = "PROCESSING"
	Complete   OrderStatus = "COMPLETE"
)

type Order struct {
	ID     int
	Type   OrderType
	Status OrderStatus
}
