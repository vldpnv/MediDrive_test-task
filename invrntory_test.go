package inventory

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestReserve_ConcurrentOversell(t *testing.T) {
	initialStock := 100
	goroutinesCount := 200
	reserveQuantity := 1

	svc := NewSafeInventoryService(map[string]*Product{
		"prod-1": {
			ID:    "prod-1",
			Name:  "Test Product",
			Stock: initialStock,
		},
	})

	var wg sync.WaitGroup
	start := make(chan struct{})

	var successfulReservations int64
	var failedReservations int64

	for i := 0; i < goroutinesCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// waiting for the starter pistol to maximize contention
			<-start

			err := svc.Reserve("prod-1", reserveQuantity)
			if err == nil {
				atomic.AddInt64(&successfulReservations, 1)
			} else if errors.Is(err, ErrInsufficientStock) {
				atomic.AddInt64(&failedReservations, 1)
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	// release all goroutines at the exact same millisecond
	close(start)
	wg.Wait()

	if successfulReservations != int64(initialStock) {
		t.Errorf("expected exactly %d successful reservations, got %d", initialStock, successfulReservations)
	}

	expectedFailures := goroutinesCount - initialStock
	if failedReservations != int64(expectedFailures) {
		t.Errorf("expected exactly %d failed reservations, got %d", expectedFailures, failedReservations)
	}

	finalStock := svc.GetStock("prod-1")
	if finalStock != 0 {
		t.Errorf("expected final stock to be 0, got %d", finalStock)
	}
}

func TestReserveMultiple_Atomicity(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"prod-A": {ID: "prod-A", Name: "Product A", Stock: 10},
		"prod-B": {ID: "prod-B", Name: "Product B", Stock: 5},
	})

	// this request asks for 8 of A (available) and 8 of B (insufficient).
	// it must fail entirely, leaving both stocks untouched.
	items := []ReserveItem{
		{ProductID: "prod-A", Quantity: 8},
		{ProductID: "prod-B", Quantity: 8},
	}

	err := svc.ReserveMultiple(items)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	// verify rollbacks (atomicity)
	stockA := svc.GetStock("prod-A")
	if stockA != 10 {
		t.Errorf("expected stock of A to remain 10, got %d", stockA)
	}

	stockB := svc.GetStock("prod-B")
	if stockB != 5 {
		t.Errorf("expected stock of B to remain 5, got %d", stockB)
	}
}

func TestReserveMultiple_DuplicateItemsInRequest(t *testing.T) {
	svc := NewSafeInventoryService(map[string]*Product{
		"prod-C": {ID: "prod-C", Name: "Product C", Stock: 10},
	})

	// requesting the same item multiple times in one batch.
	// total requested is 6 + 5 = 11, which exceeds stock of 10.
	items := []ReserveItem{
		{ProductID: "prod-C", Quantity: 6},
		{ProductID: "prod-C", Quantity: 5},
	}

	err := svc.ReserveMultiple(items)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock due to aggregated quantity, got %v", err)
	}

	stockC := svc.GetStock("prod-C")
	if stockC != 10 {
		t.Errorf("expected stock of C to remain 10, got %d", stockC)
	}
}
