package repository

import (
	"context"
	"database/sql"
	"domainconcern/sampleApp/domain"
	"domainconcern/utils/money"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type OrderRepository interface {
	Save(ctx context.Context, order *domain.Order) error
	FindByID(ctx context.Context, id string) (*domain.Order, error)
}

type PostgresOrderRepository struct {
	db *sql.DB
}

func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

func (r *PostgresOrderRepository) Save(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
        INSERT INTO orders (id, customer_id, total_amount, total_currency, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (id) DO UPDATE SET
            customer_id = EXCLUDED.customer_id,
            total_amount = EXCLUDED.total_amount,
            total_currency = EXCLUDED.total_currency,
            status = EXCLUDED.status,
            updated_at = EXCLUDED.updated_at
    `, order.ID, order.CustomerID, order.Total.Amount(), order.Total.Currency(),
		order.Status, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save order: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM order_items WHERE order_id = $1`, order.ID)
	if err != nil {
		return fmt.Errorf("delete old items: %w", err)
	}

	for _, item := range order.Items {
		_, err = tx.ExecContext(ctx, `
            INSERT INTO order_items (order_id, product_id, quantity, price_amount, price_currency)
            VALUES ($1, $2, $3, $4, $5)
        `, order.ID, item.ProductID, item.Quantity, item.Price.Amount(), item.Price.Currency())
		if err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}
	return tx.Commit()
}

func (r *PostgresOrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	var order domain.Order
	var totalAmount decimal.Decimal
	var totalCurrency, status string
	var createdAt, updatedAt time.Time

	row := r.db.QueryRowContext(ctx, `
        SELECT id, customer_id, total_amount, total_currency, status, created_at, updated_at
        FROM orders WHERE id = $1
    `, id)
	err := row.Scan(&order.ID, &order.CustomerID, &totalAmount, &totalCurrency, &status, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	order.Total, err = money.NewMoney(totalAmount, totalCurrency)
	if err != nil {
		return nil, err
	}
	order.Status = domain.OrderStatus(status)
	order.CreatedAt = createdAt
	order.UpdatedAt = updatedAt

	rows, err := r.db.QueryContext(ctx, `
        SELECT product_id, quantity, price_amount, price_currency
        FROM order_items WHERE order_id = $1
    `, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.OrderItem
		var priceAmount decimal.Decimal
		var priceCurrency string
		if err := rows.Scan(&item.ProductID, &item.Quantity, &priceAmount, &priceCurrency); err != nil {
			return nil, err
		}
		item.Price, err = money.NewMoney(priceAmount, priceCurrency)
		if err != nil {
			return nil, err
		}
		order.Items = append(order.Items, item)
	}
	return &order, nil
}
