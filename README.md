
# 🚀 SplitCore — Telegram Expense Organizer

[![SplitCore CI](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml/badge.svg)](https://github.com/GanFay/SplitCore/actions/workflows/ci.yml)

**SplitCore** is a Telegram bot designed to automate shared expense tracking for groups of friends, travelers, or event organizers. No more messy Excel sheets or "who owes whom" arguments.

## 🔥 The Core Idea
Users create "Funds" (events), invite friends via unique deep-links, and record their expenses. The bot automatically calculates the balance: who overpaid and who needs to settle their debt using a greedy matching algorithm to minimize transactions.

### 🎬 Demo
*(Note: For the best experience, view the `.mp4` video directly if the GIF is buffering)*

![SplitCore Demo](demo2.gif)

## 🛠 Tech Stack
* **Language:** Go (Golang) 1.26.2
* **Framework:**[telebot.v4](https://github.com/tucnak/telebot) (Telegram Bot API)
* **Database:** PostgreSQL
* **Driver:** [pgx/v5](https://github.com/jackc/pgx) (Connection Pool)
* **State Management:** Redis 8.6.2 via [go-redis/v9](https://github.com/redis/go-redis) for persistent FSM.
* **Infrastructure:** Docker, Docker Compose, Makefile.
* **Migrations:** [golang-migrate](https://github.com/golang-migrate/migrate).

## 🏗 Architecture (Clean Architecture)
The project is built with a strict separation of concerns, ensuring high testability and scalability:
- `cmd/bot/` — Entry point, initialization, and Dependency Injection.
- `internal/domain/` — Business entities, Data models, and Core interfaces.
- `internal/repository/` — Database access layer (PostgreSQL and Redis implementations).
- `internal/usecase/` — Core business logic, math calculations, and data processing.
- `internal/delivery/telegram/` — Bot-specific UI logic (handlers, menus, router).
- `internal/pkg/` — Internal utilities (e.g., deep-link generators, JSON encoders).

## 📍 Roadmap

### ✅ MVP (Completed)
- [x] Clean Architecture setup and Dependency Injection.
- [x] PostgreSQL integration with migrations.
- [x] FSM for user input handling and seamless UX.
- [x] Fund creation and unique Deep-Link generation.
- [x] Expense logging and history tracking.
- [x] Advanced debt calculation algorithm (Settle Up).
- [x] Move FSM states from in-memory to Redis for persistence and horizontal scaling.
- [x] Graceful shutdown implementation.
- [x] Table-Driven Unit Tests for the settlement math module.
- [x] CI/CD Pipeline (GitHub Actions + golangci-lint).

### 🌟V1.1.0 
- [x] Virtual Users (Add members without Telegram accounts).

### V1.1.1
- [x] Deploy to VPS (DigitalOcean).

### V1.1.2
- [x] **Secure Fund Deletion:** Creator-only access with FSM confirmation state to prevent accidental wipes.

### 🚀 Enhancements (Future)
- [ ] **Expense Management:** Ability to delete or edit logged mistakes.
- [ ] **Settle Debt feature:** "Mark as paid" logic to automatically adjust balances when someone returns the money.
- [ ] **Export to CSV:** Generate and download fund reports on the fly.
- [ ] **Multi-currency support.**

## ⚙️ Getting Started (Dev)

**Prerequisites:** Docker, Docker Compose, and Make.

1. Clone the repository:
   ```bash
   git clone https://github.com/GanFay/SplitCore.git
   ```
2. Set up environment variables. Copy the example file and fill in your details (Bot token, DB credentials):
   ```bash
   cp .env.example .env
   ```
3. Start the database and Redis using Makefile:
   ```bash
   make services-run
   ```
4. Run migrations:
   ```bash
   make migrate-up
   ```
