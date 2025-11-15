package network

type IHandler interface {
	OnOpen(entity IEntity) error
	OnTraffic(entity IEntity) error
	OnClose(entity IEntity, _ error) error
}
