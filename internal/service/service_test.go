package service

import "testing"

func TestReservePreventsOverlapByDefault(t *testing.T) {
	service := &Service{running: make(map[string]int)}
	if !service.reserve("task", false) {
		t.Fatal("first run should reserve")
	}
	if service.reserve("task", false) {
		t.Fatal("second non-overlapping run should be rejected")
	}
	if !service.reserve("task", true) {
		t.Fatal("explicit overlapping run should reserve")
	}
	service.release("task")
	service.release("task")
	if len(service.running) != 0 {
		t.Fatalf("running map not cleared: %v", service.running)
	}
}
