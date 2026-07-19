package inventory

import (
	"errors"
	"sync"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Product struct {
	ID    string
	Name  string
	Stock int
}

type ReserveItem struct {
	ProductID string
	Quantity  int
}

type SafeInventoryService struct {
	mu       sync.RWMutex
	products map[string]*Product
}

func NewSafeInventoryService(initialProducts map[string]*Product) *SafeInventoryService {
	if initialProducts == nil {
		initialProducts = make(map[string]*Product)
	}
	return &SafeInventoryService{
		products: initialProducts,
	}
}

func (s *SafeInventoryService) GetStock(productID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product := s.products[productID]
	if product == nil {
		return 0
	}
	return product.Stock
}

func (s *SafeInventoryService) Reserve(productID string, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	product := s.products[productID]
	if product == nil {
		return ErrProductNotFound
	}

	if product.Stock < quantity {
		return ErrInsufficientStock
	}

	product.Stock -= quantity
	return nil
}

func (s *SafeInventoryService) ReserveMultiple(items []ReserveItem) error {
	if len(items) == 0 {
		return nil
	}

	// NOTE:we must aggregate quantities first. If the caller passes the same ProductID
	// multiple times in a single batch, a naive loop will cause a double-deduction bug
	// and bypass stock limits. Doing this to avoid weird edge cases.
	requiredQuantities := make(map[string]int)
	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}
		requiredQuantities[item.ProductID] += item.Quantity
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for productID, qty := range requiredQuantities {
		product := s.products[productID]
		if product == nil {
			return ErrProductNotFound
		}
		if product.Stock < qty {
			return ErrInsufficientStock
		}
	}

	for productID, qty := range requiredQuantities {
		s.products[productID].Stock -= qty
	}

	return nil
}
