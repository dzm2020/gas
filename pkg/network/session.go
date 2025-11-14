package network

type ISession interface {
	OnConnect(entity IEntity) error
	OnTraffic() error
	OnClose(err error) error
}

var _ ISession = (*Session)(nil)

type Session struct {
}

func (a *Session) OnConnect(entity IEntity) error {
	return nil
}

func (a *Session) OnTraffic() error {
	return nil
}

func (a *Session) OnClose(err error) error {
	return nil
}
