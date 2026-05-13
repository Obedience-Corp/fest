package watch

import (
	"context"
	"errors"
)

type target struct {
	selector string
}

type runner struct{}

func newRunner() runner {
	return runner{}
}

func (runner) Resolve(ctx context.Context, selector string) (target, error) {
	if err := ctx.Err(); err != nil {
		return target{}, err
	}
	return target{selector: selector}, nil
}

func (runner) Watch(ctx context.Context, _ target, _ options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("fest watch is not implemented yet")
}
