package l4la

type Addr struct{}

func (a *Addr) Network() string {
	return "l4la"
}

func (a *Addr) String() string {
	return "l4la"
}
