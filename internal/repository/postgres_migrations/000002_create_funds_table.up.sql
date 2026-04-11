CREATE TABLE IF NOT EXISTS funds (
                                    id SERIAL PRIMARY KEY,
                                    name TEXT NOT NULL,
                                    author_id BIGINT REFERENCES users(tg_id),
                                    invite_code TEXT UNIQUE NOT NULL,
                                    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);