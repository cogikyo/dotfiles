package ui

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSpinJoinsWorkerOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var returned atomic.Bool
	time.AfterFunc(20*time.Millisecond, cancel)
	err := spin(ctx, &bytes.Buffer{}, "work", func(ctx context.Context) error {
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		returned.Store(true)
		return ctx.Err()
	})
	if !returned.Load() {
		t.Fatal("spin returned before its worker")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
}

func TestSpinReturnsWorkError(t *testing.T) {
	want := errors.New("boom")
	if err := spin(context.Background(), &bytes.Buffer{}, "work", func(context.Context) error { return want }); !errors.Is(err, want) {
		t.Fatalf("err %v, want %v", err, want)
	}
}
