-- +goose Up
-- +goose StatementBegin
CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    number VARCHAR(255) NOT NULL,
    status order_status NOT NULL DEFAULT 'NEW',
    accrual DECIMAL(10,2),
    uploaded_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT orders_number_unique UNIQUE (number)
);
-- +goose StatementEnd

-- +goose Down
DROP TABLE orders;
DROP TYPE order_status;
