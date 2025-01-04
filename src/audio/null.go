package audio

import (
	"io"
)

type NullBackend struct{}

func NewNullBackend() (NullBackend, error) {
	return NullBackend{}, nil
}

func (p NullBackend) GetVolume() float64 {
	return 0
}

func (p NullBackend) SetVolume(level float64) {
}

func (p NullBackend) BufSize() int {
	return 0
}

func (p NullBackend) Output() io.Writer {
	return nil
}
