-- +goose Up
CREATE TABLE balances (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    current DECIMAL(10,2) NOT NULL DEFAULT 0,
    withdrawn DECIMAL(10,2) NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE balances;
