CREATE TABLE IF NOT EXISTS customers (
    id INTEGER PRIMARY KEY,
    full_name VARCHAR(160) NOT NULL,
    province VARCHAR(80) NOT NULL,
    city VARCHAR(80) NOT NULL,
    segment VARCHAR(40) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sales_orders (
    id INTEGER PRIMARY KEY,
    order_date DATE NOT NULL,
    customer_id INTEGER NOT NULL REFERENCES customers(id),
    province VARCHAR(80) NOT NULL,
    city VARCHAR(80) NOT NULL,
    product_category VARCHAR(80) NOT NULL,
    product_name VARCHAR(140) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(14, 2) NOT NULL CHECK (unit_price >= 0),
    total_amount NUMERIC(14, 2) NOT NULL CHECK (total_amount >= 0),
    payment_method VARCHAR(40) NOT NULL,
    status VARCHAR(40) NOT NULL
);

CREATE TABLE IF NOT EXISTS monthly_kpis (
    id INTEGER PRIMARY KEY,
    month DATE NOT NULL,
    province VARCHAR(80) NOT NULL,
    total_sales NUMERIC(16, 2) NOT NULL,
    total_orders INTEGER NOT NULL,
    active_customers INTEGER NOT NULL,
    average_order_value NUMERIC(14, 2) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sales_orders_order_date ON sales_orders(order_date);
CREATE INDEX IF NOT EXISTS idx_sales_orders_province ON sales_orders(province);
CREATE INDEX IF NOT EXISTS idx_sales_orders_category ON sales_orders(product_category);
CREATE INDEX IF NOT EXISTS idx_sales_orders_status ON sales_orders(status);
CREATE INDEX IF NOT EXISTS idx_customers_segment ON customers(segment);
CREATE INDEX IF NOT EXISTS idx_monthly_kpis_month ON monthly_kpis(month);

