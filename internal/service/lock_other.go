//go:build !unix

package service

type Lock struct{}

func AcquireLock(string) (*Lock, error) { return &Lock{}, nil }
func (l *Lock) Close() error            { return nil }
