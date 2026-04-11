CREATE TABLE IF NOT EXISTS purchases (
    id SERIAL PRIMARY KEY,
    fund_id INTEGER REFERENCES funds(id) ON DELETE CASCADE,
    payer_id BIGINT REFERENCES users(tg_id),
    amount NUMERIC(10, 2) NOT NULL, -- Сумма (используем numeric для точности денег)
    description TEXT,               -- Что купили (мясо, билеты, угли)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);