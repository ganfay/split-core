CREATE SCHEMA app;


CREATE TABLE IF NOT EXISTS app.users (
                                         tg_id BIGINT PRIMARY KEY,
                                         username TEXT,
                                         first_name TEXT NOT NULL,
                                         created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS app.funds (
                                         id SERIAL PRIMARY KEY,
                                         name TEXT NOT NULL,
                                         author_id BIGINT REFERENCES app.users(tg_id),
                                         invite_code TEXT UNIQUE NOT NULL,
                                         created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS app.fund_members (
                                            fund_id BIGINT REFERENCES app.funds(id) ON DELETE CASCADE,
                                            user_id BIGINT REFERENCES app.users(tg_id) ON DELETE CASCADE,
                                            PRIMARY KEY (fund_id, user_id)
);

CREATE TABLE IF NOT EXISTS app.purchases (
                                             id SERIAL PRIMARY KEY,
                                             fund_id INTEGER REFERENCES app.funds(id) ON DELETE CASCADE,
                                             payer_id BIGINT REFERENCES app.users(tg_id),
                                             amount NUMERIC(10, 2) NOT NULL,
                                             description TEXT,
                                             created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
