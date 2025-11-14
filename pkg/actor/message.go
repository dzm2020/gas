package actor

type InitMessage struct{}

type StopMessage struct{}

type AsyncCallMessage struct {
	f func() error
}

type Message struct {
	Name string
	Args []interface{}
}
