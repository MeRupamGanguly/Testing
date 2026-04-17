CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    total_amount NUMERIC(20,8) NOT NULL,
    total_currency TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE TABLE IF NOT EXISTS order_items (
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL,
    quantity INT NOT NULL,
    price_amount NUMERIC(20,8) NOT NULL,
    price_currency TEXT NOT NULL,
    PRIMARY KEY (order_id, product_id)
);
