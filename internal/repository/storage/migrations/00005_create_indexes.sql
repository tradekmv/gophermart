-- +goose Up
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_status_user ON orders(status, user_id);
CREATE INDEX idx_withdrawals_user_id ON withdrawals(user_id);

-- +goose Down
DROP INDEX idx_withdrawals_user_id;
DROP INDEX idx_orders_status_user;
DROP INDEX idx_orders_status;
DROP INDEX idx_orders_user_id;
