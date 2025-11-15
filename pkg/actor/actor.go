package actor

type (
	Producer func() IActor

	IActor interface {
		OnInit(ctx IContext, params []interface{}) error
		OnStop(ctx IContext) error
	}
)

var _ IActor = (*Actor)(nil)

type Actor struct {
}

func (a *Actor) OnInit(ctx IContext, params []interface{}) error {
	return nil
}
func (a *Actor) OnStop(ctx IContext) error {
	return nil
}
