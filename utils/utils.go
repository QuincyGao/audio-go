package utils

type TailBuffer struct {
	Limit int
	data  []byte
}

func (b *TailBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	b.data = append(b.data, p...)
	if len(b.data) > b.Limit {
		b.data = b.data[len(b.data)-b.Limit:]
	}
	return n, nil
}

func (b *TailBuffer) String() string {
	return string(b.data)
}
