CREATE TABLE IF NOT EXISTS fund_members (
                                    fund_id BIGINT REFERENCES funds(id) ON DELETE CASCADE,
                                    user_id BIGINT REFERENCES users(tg_id) ON DELETE CASCADE,
                                    PRIMARY KEY (fund_id, user_id)
);