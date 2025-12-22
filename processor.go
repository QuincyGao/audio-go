package audiogo

import "context"

type Processor interface {
	Init(context.Context) error
	Run() error
	Wait() error
	Done()

	WriteTo(int, []byte) error
	ReadFrom(int, []byte) (int, error)
	CloseInput()
}
