package service

import (
	"context"
	"domainconcern/sampleApp/domain"
	repository "domainconcern/sampleApp/repostory"
	validation "domainconcern/utils"

	"domainconcern/utils/id"
	"domainconcern/utils/money"
	"domainconcern/utils/timeutil"
	"errors"

	"github.com/shopspring/decimal"
)

type OrderService struct {
	repo            repository.OrderRepository
	defaultCurrency string
}

func NewOrderService(repo repository.OrderRepository, defaultCurrency string) *OrderService {
	return &OrderService{repo: repo, defaultCurrency: defaultCurrency}
}

type CreateOrderRequest struct {
	CustomerID string
	Items      []struct {
		ProductID string
		Quantity  int
		Price     string
		Currency  string
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*domain.Order, error) {
	if err := validation.ValidateNonZeroString(req.CustomerID, "customerID"); err != nil {
		return nil, err
	}
	if len(req.Items) == 0 {
		return nil, errors.New("order must contain at least one item")
	}

	var items []domain.OrderItem
	var total money.Money
	var firstCurrency string

	for i, it := range req.Items {
		if it.Quantity <= 0 {
			return nil, validation.ValidatePositive(int64(it.Quantity), "item.quantity")
		}
		if err := validation.ValidateNonZeroString(it.ProductID, "productID"); err != nil {
			return nil, err
		}

		amt, err := decimal.NewFromString(it.Price)
		if err != nil {
			return nil, errors.New("invalid price format")
		}
		currency := it.Currency
		if currency == "" {
			currency = s.defaultCurrency
		}
		priceMoney, err := money.NewMoney(amt, currency)
		if err != nil {
			return nil, err
		}

		if i == 0 {
			firstCurrency = priceMoney.Currency()
		} else if priceMoney.Currency() != firstCurrency {
			return nil, errors.New("all order items must share the same currency")
		}

		itemTotal := priceMoney.Mul(money.MustNewMoneyFromFloat(float64(it.Quantity), firstCurrency).Amount())
		if i == 0 {
			total = itemTotal
		} else {
			total, _ = total.Add(itemTotal)
		}
		items = append(items, domain.OrderItem{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
			Price:     priceMoney,
		})
	}

	total = total.Round()
	now := timeutil.NowUTC()
	order := &domain.Order{
		ID:         id.NewID(),
		CustomerID: req.CustomerID,
		Items:      items,
		Total:      total,
		Status:     domain.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	if err := validation.ValidateNonZeroString(id, "orderID"); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

func (s *OrderService) ConfirmOrder(ctx context.Context, id string) error {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if order.Status != domain.StatusPending {
		return errors.New("only pending orders can be confirmed")
	}
	order.Status = domain.StatusConfirmed
	order.UpdatedAt = timeutil.NowUTC()
	return s.repo.Save(ctx, order)
}

func (s *OrderService) CancelOrder(ctx context.Context, id string) error {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if order.Status != domain.StatusPending {
		return errors.New("only pending orders can be cancelled")
	}
	order.Status = domain.StatusCancelled
	order.UpdatedAt = timeutil.NowUTC()
	return s.repo.Save(ctx, order)
}
