-- +goose Up
CREATE TABLE withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    "order" VARCHAR(255) NOT NULL,
    sum DECIMAL(10,2) NOT NULL,
    processed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE withdrawals;
