# makeMe — План архитектуры

## Реализованные фичи (changelog)

- [DONE] **`/words` UX-доработка:** убрана кнопка «⏹ Завершить» и подпись «Слово N/M» из заголовка слота (последняя — спойлер: позволяла угадывать слово по серийному номеру). Добавлена адаптивная кнопка «⏭ Отложить до завтра» в feedback после 6+ ошибок на слово в сессии — слово исключается из текущей сессии, прогресс сохраняется, на следующий запуск возвращается через `GetInProgressWordsByLevel`. Новые колонки в `words_sessions`: `error_counts JSONB`, `deferred_word_ids BIGINT[]`.

- [DONE] **`/words` — режим активного заучивания слов.** 10 слов на сессию (5 текущего + 5 следующего уровня), 4 режима (EN→RU MC, RU→EN MC, EN→RU typing, RU→EN typing), порог зачёта режима — 2 правильных ответа, ошибка сбрасывает счётчик. Слово после выучивания автоматически уходит в SM-2 повторение через `review_cards`. Не идёт в зачёт лидерборда. Без дневного лимита на сессии. Параллельно: typing-режим добавлен в `/tasks` как 4-й и 5-й vocab-subtype (`typing_en_ru`, `typing_ru_en`) — там он зачётный. Новые таблицы: `word_learning_progress`, `words_sessions`, `pending_typing_prompts`. Доменные пакеты: `internal/domain/words`, `internal/textmatch`.

## Контекст

Проект makeMe — на стадии Hello World. Нужно заложить архитектуру для геймифицированной платформы изучения английского через Telegram-бота. Стек определён: Go + chi + PostgreSQL (pgx/sqlc/goose) + go-telegram/bot.

---

## Структура каталогов

```
makeMe/
├── cmd/
│   └── makeme/
│       └── main.go                  # Точка входа: загрузка конфига, создание зависимостей, запуск сервера
│
├── internal/
│   ├── config/
│   │   └── config.go                # Загрузка конфигурации из переменных окружения + .env (godotenv)
│   │
│   ├── domain/                      # Бизнес-логика, не зависит от инфраструктуры
│   │   ├── user/
│   │   │   ├── entity.go            # Сущности: User, Level (A1-C2)
│   │   │   ├── repository.go        # Интерфейс репозитория
│   │   │   └── service.go           # Регистрация, управление уровнем
│   │   │
│   │   ├── group/
│   │   │   ├── entity.go            # Сущности: Group, Membership, MemberRole
│   │   │   ├── repository.go        # Интерфейс репозитория
│   │   │   └── service.go           # Создание группы, вступление, инвайт-коды
│   │   │
│   │   ├── task/
│   │   │   ├── entity.go            # Сущности: Task, DailySet, TaskResult, TaskType
│   │   │   ├── repository.go        # Интерфейс репозитория
│   │   │   ├── service.go           # Оркестрация ежедневной генерации заданий
│   │   │   ├── generator.go         # Интерфейс TaskGenerator (точка расширения)
│   │   │   ├── vocab.go             # VocabGenerator — задания на словарный запас (использует SRS)
│   │   │   ├── grammar.go           # GrammarGenerator — грамматические задания
│   │   │   └── translation.go       # TranslationGenerator — задания на перевод
│   │   │
│   │   ├── srs/
│   │   │   ├── entity.go            # ReviewCard — карточка с полями SM-2
│   │   │   ├── repository.go        # Интерфейс репозитория
│   │   │   └── sm2.go               # Алгоритм SM-2, чистые функции без зависимостей
│   │   │
│   │   ├── scoring/
│   │   │   ├── entity.go            # Score, LeaderboardEntry
│   │   │   ├── repository.go        # Интерфейс репозитория
│   │   │   └── service.go           # Расчёт баллов, нормализация, таблицы лидеров
│   │   │
│   │   └── assessment/
│   │       ├── entity.go            # AssessmentSession, Answer
│   │       ├── repository.go        # Интерфейс репозитория
│   │       └── service.go           # Логика оценки уровня, адаптивный подбор вопросов
│   │
│   ├── storage/                     # Реализации репозиториев (инфраструктурный слой)
│   │   ├── postgres/
│   │   │   ├── postgres.go          # Создание пула соединений pgx
│   │   │   ├── user.go              # UserRepository реализация
│   │   │   ├── group.go             # GroupRepository реализация
│   │   │   ├── task.go              # TaskRepository реализация
│   │   │   ├── srs.go               # SRS ReviewCardRepository реализация
│   │   │   ├── scoring.go           # ScoreRepository реализация
│   │   │   └── assessment.go        # AssessmentRepository реализация
│   │   │
│   │   └── sqlc/                    # Код, сгенерированный sqlc (НЕ редактировать вручную)
│   │       ├── db.go
│   │       ├── models.go
│   │       └── queries.go
│   │
│   ├── telegram/                    # Тонкий адаптер: парсит update → вызывает домен → форматирует ответ
│   │   ├── bot.go                   # Инициализация бота, webhook, регистрация хендлеров
│   │   ├── handler_start.go         # /start — регистрация + запуск оценки
│   │   ├── handler_task.go          # Отображение заданий, приём ответов
│   │   ├── handler_group.go         # Управление группами
│   │   ├── handler_leaderboard.go   # Таблицы лидеров
│   │   ├── handler_profile.go       # Профиль и статистика
│   │   ├── handler_assessment.go    # Диалог оценки уровня
│   │   ├── keyboard.go             # Построители клавиатур (go-telegram/ui)
│   │   └── middleware.go            # Middleware: определение пользователя по telegram_id
│   │
│   ├── scheduler/
│   │   └── scheduler.go             # Крон: ежедневная генерация заданий, отправка итогов
│   │
│   └── server/
│       └── server.go                # chi-роутер, маршрут webhook, /health
│
├── migrations/                      # SQL-миграции goose
│   ├── 00001_create_users.sql
│   ├── 00002_create_groups.sql
│   ├── 00003_create_tasks.sql
│   ├── 00004_create_srs.sql
│   ├── 00005_create_scoring.sql
│   └── 00006_create_assessment.sql
│
├── queries/                         # SQL-запросы для sqlc
│   ├── users.sql
│   ├── groups.sql
│   ├── tasks.sql
│   ├── srs.sql
│   ├── scoring.sql
│   └── assessment.sql
│
├── sqlc.yaml                        # Конфигурация sqlc
├── .env.example                     # Пример переменных окружения
├── Makefile                         # make run, migrate-up, migrate-down, sqlc, build
└── CLAUDE.md
```

---

## Доменные сущности и связи

### User (пользователь)
- `id`, `telegram_id` (уникальный), `username`, `first_name`
- `level` — текущий уровень (A1/A2/B1/B2/C1/C2)
- `onboarded` — прошёл ли начальную оценку
- Связи: состоит в группах (через Membership), имеет карточки SRS, результаты заданий, баллы

### Group (группа) + Membership (участие)
- Group: `id`, `name`, `invite_code` (уникальный короткий код), `created_by`
- Membership: `group_id`, `user_id`, `role` (owner/member)
- Один пользователь может быть в нескольких группах
- У каждой группы свой лидерборд

### DailySet (набор заданий на день) + Task (задание)
- DailySet: `user_id` + `group_id` + `date` (уникальная комбинация)
- Task: `type` (vocab/grammar/translation), `level`, `prompt`, `options` (JSON, null для свободного ввода), `answer`, `metadata` (JSON)
- Разные типы заданий хранят разную структуру в `prompt`/`metadata` — это обеспечивает гибкость без изменения схемы

### TaskResult (результат задания)
- Один результат на задание на пользователя
- `response`, `correct`, `score` (нормализованные баллы), `duration_ms`

### ReviewCard (карточка SRS)
- Связь пользователь ↔ слово, поля SM-2: `ease_factor`, `interval_days`, `repetitions`, `next_review_date`
- Таблица `words`: `english`, `russian`, `level`, `example`

### Score (баллы)
- Один рекорд на пользователя + группу + день
- `raw` (сырые баллы) и `normalized` (после нормализации по уровню)

### AssessmentSession (сессия оценки)
- Статус: in_progress / completed
- По завершении определяет `level` пользователя
- Содержит AssessmentAnswer с вопросами разных уровней

---

## Нормализация баллов (справедливость соревнования)

Формула: `normalized = raw × coefficient(level)`

| Уровень | Коэффициент | Смысл |
|---------|------------|-------|
| A1      | 0.6        | Лёгкие задания → меньший множитель |
| A2      | 0.8        | |
| B1      | 1.0        | Базовый уровень |
| B2      | 1.15       | |
| C1      | 1.3        | |
| C2      | 1.5        | Сложные задания → больший множитель |

Если A1 и C2 пользователи отвечают на все свои задания идеально — они получают сопоставимые итоговые баллы.

---

## Расширяемость заданий (TaskGenerator)

Интерфейс:
```go
type TaskGenerator interface {
    Type() TaskType
    Generate(ctx context.Context, userID int64, level user.Level, count int) ([]Task, error)
}
```

Чтобы добавить новый тип задания (например, аудирование):
1. Добавить константу `TypeListening` в `TaskType`
2. Создать `internal/domain/task/listening.go`, реализующий `TaskGenerator`
3. Зарегистрировать генератор в `main.go` при создании `task.Service`
4. Миграция: добавить значение в CHECK constraint

---

## Интеграция Telegram-бота с chi

```go
// cmd/makeme/main.go — упрощённо
r := chi.NewRouter()
r.Use(middleware.Logger, middleware.Recoverer)
r.Get("/health", healthHandler)
r.Post("/webhook/{token}", tgBot.WebhookHandler()) // стандартный http.HandlerFunc из go-telegram/bot

http.ListenAndServe(":"+cfg.Port, r)
```

Telegram-хендлеры — тонкие адаптеры:
- Парсят данные из Update
- Вызывают доменные сервисы
- Форматируют ответ и отправляют в Telegram
- Никакой бизнес-логики в telegram-пакете

---

## Конфигурация

```go
type Config struct {
    Port            string // "8080"
    DatabaseURL     string // "postgres://user:pass@host:5432/makeme"
    TelegramToken   string
    WebhookURL      string // "https://domain.com/webhook/{token}"
    LogLevel        string // "debug" / "info" / "warn" / "error"
    MaxRoundsPerDay int    // limit on /tasks rounds per user per day (default 5)
}
```

Загрузка: `godotenv` читает `.env`, затем `os.Getenv` для каждого поля.

---

## План реализации по фазам (MVP)

### Фаза 1 — Скелет и инфраструктура [DONE]

**Цель:** запускаемое приложение с подключением к БД и health-эндпоинтом.

Что сделано:
- [x] `cmd/makeme/main.go` — точка входа, загрузка конфига, создание pgx-пула, chi-роутер, graceful shutdown, запуск HTTP-сервера
- [x] `internal/config/config.go` — загрузка из env + godotenv
- [x] `internal/storage/postgres/postgres.go` — создание пула pgx
- [x] `internal/server/server.go` — chi-роутер с `/health`
- [x] `migrations/00001_create_users.sql` — первая миграция
- [x] `sqlc.yaml` + `queries/users.sql` — конфигурация sqlc + запросы для users
- [x] `Makefile` с целями: `run`, `build`, `migrate-up`, `migrate-down`, `migrate-create`, `sqlc`
- [x] `.env.example` — шаблон переменных окружения
- [x] Структура каталогов создана полностью
- [x] Зависимости установлены: chi, pgx, godotenv
- [x] `go build` и `go vet` проходят без ошибок

**Проверка:** `make migrate-up` без ошибок, `make run` запускает сервер, `curl localhost:8080/health` → 200 OK.

---

### Фаза 2 — Telegram-бот + регистрация пользователей [DONE]

**Цель:** бот отвечает на `/start` и создаёт пользователя в БД.

Что сделано:
- [x] `internal/domain/user/entity.go` — сущность User, тип Level (A1-C2)
- [x] `internal/domain/user/repository.go` — интерфейс: `Create`, `GetByTelegramID`, `GetByID`, `UpdateLevel`
- [x] `internal/domain/user/service.go` — `Register` (идемпотентная), `GetByTelegramID`, `UpdateLevel`
- [x] `internal/storage/postgres/user.go` — реализация репозитория через sqlc, маппинг sqlc→domain
- [x] `internal/storage/sqlc/` — сгенерированный код sqlc
- [x] `internal/telegram/bot.go` — инициализация `go-telegram/bot`, webhook handler, SetWebhook, StartWebhook
- [x] `internal/telegram/handler_start.go` — обработчик `/start`: регистрирует пользователя, приветствие зависит от onboarded
- [x] `internal/server/server.go` — добавлен маршрут `POST /webhook`
- [x] `cmd/makeme/main.go` — подключены: sqlc queries → userRepo → userSvc → tgBot → server
- [x] Зависимость `github.com/go-telegram/bot` установлена
- [x] `go build` и `go vet` проходят без ошибок
- [ ] `internal/telegram/middleware.go` — отложен (будет нужен когда появятся другие команды)

**Проверка:** отправить `/start` боту → бот отвечает приветствием, в таблице `users` появилась строка.

---

### Фаза 3 — Группы [DONE]

**Цель:** пользователи могут создавать группы и вступать в них по инвайт-коду.

Что сделано:
- [x] `migrations/00002_create_groups.sql` — таблицы `groups` и `memberships`
- [x] `internal/domain/group/entity.go` — Group, Membership, MemberRole
- [x] `internal/domain/group/repository.go` — интерфейс: `Create`, `AddMember`, `GetByInviteCode`, `ListByUser`, `IsMember`
- [x] `internal/domain/group/service.go` — создание группы (генерация invite_code), вступление (с проверкой дублей), список групп
- [x] `internal/storage/postgres/group.go` — реализация через sqlc
- [x] `queries/groups.sql` — SQL-запросы
- [x] `internal/telegram/handler_group.go` — `/newgroup`, `/join`, `/mygroups`
- [x] `internal/telegram/middleware.go` — user-resolve middleware (Register + context)
- [x] `internal/telegram/bot.go` — добавлен groupSvc, middleware, регистрация команд
- [x] `cmd/makeme/main.go` — подключён groupSvc
- [x] `go build` и `go vet` без ошибок

**Проверка:** создать группу → получить инвайт-код → другой пользователь вступает по коду → оба видят группу в `/mygroups`.

---

### Фаза 4 — Оценка уровня (Assessment) [DONE]

**Цель:** новый пользователь проходит тест и получает уровень A1–C2.

Что сделано:
- [x] `migrations/00003_create_assessment.sql` — таблицы `assessment_sessions`, `assessment_answers`
- [x] `internal/domain/assessment/entity.go` — Session, Answer, Question, SessionStatus
- [x] `internal/domain/assessment/repository.go` — интерфейс: `CreateSession`, `GetActiveSession`, `UpdateSession`, `AddAnswer`, `CountAnswers`, `CountCorrectByLevel`
- [x] `internal/domain/assessment/questions.go` — банк вопросов: 30 вопросов (5 на каждый уровень A1–C2)
- [x] `internal/domain/assessment/service.go` — адаптивная логика:
  - Начинает с B1, при правильном ответе повышает уровень, при неправильном — понижает
  - После 10 вопросов определяет уровень (наивысший с ≥50% правильных)
  - Возобновляет прерванную сессию
- [x] `internal/storage/postgres/assessment.go` — реализация через sqlc
- [x] `queries/assessment.sql` — SQL-запросы
- [x] `internal/telegram/handler_assessment.go` — `/assess` + inline-клавиатура для ответов
- [x] `internal/telegram/bot.go` — добавлен assessSvc, регистрация `/assess` и callback `assess:`
- [x] `cmd/makeme/main.go` — подключён assessSvc
- [x] `internal/domain/assessment/service_test.go` — unit-тесты: `adjustLevel`, `determineLevel`
- [x] `internal/telegram/handler_assessment_test.go` — integration-тесты: старт, онбордженный, полный flow, возобновление
- [x] `go build` и `go vet` без ошибок

**Проверка:** `/start` → бот предлагает пройти оценку → 10-15 вопросов → бот сообщает определённый уровень → в БД обновлён `level` и `onboarded = true`.

---

### Фаза 5 — Задания + SRS (ядро продукта) [DONE]

**Цель:** ежедневная генерация заданий, выполнение через бота, интервальное повторение слов.

Что сделано:
- [x] `migrations/00004_create_srs.sql` — таблицы `words`, `review_cards`
- [x] `migrations/00005_create_tasks.sql` — таблицы `daily_sets`, `tasks`, `task_results`
- [x] `migrations/00006_seed_words.sql` — 60 слов (10 на каждый уровень A1–C2)
- [x] `internal/domain/srs/entity.go` — Word, ReviewCard
- [x] `internal/domain/srs/sm2.go` — алгоритм SM-2 (чистая функция): вход — карточка + качество (0–5), выход — обновлённая карточка
- [x] `internal/domain/srs/repository.go` — интерфейс: `GetDueCards`, `UpsertCard`, `GetWordsByLevel`, `GetWordByID`, `GetNewWordsForUser`
- [x] `internal/domain/task/entity.go` — Task, DailySet, TaskResult, TaskType (vocab/grammar)
- [x] `internal/domain/task/repository.go` — интерфейс: `CreateDailySet`, `GetDailySet`, `AddTask`, `GetTasksByDailySet`, `GetTaskByID`, `AddResult`, `GetResult`, `CountResults`
- [x] `internal/domain/task/generator.go` — интерфейс `TaskGenerator`
- [x] `internal/domain/task/vocab.go` — VocabGenerator: due review cards + новые слова, 4 варианта ответа
- [x] `internal/domain/task/grammar.go` — GrammarGenerator: 30 грамматических вопросов (5 на уровень)
- [x] `internal/domain/task/service.go` — GenerateDaily (идемпотентная), SubmitAnswer (+ SRS update), GetNextUnanswered, GetProgress
- [x] `internal/storage/postgres/srs.go` — реализация SRS-репозитория через sqlc
- [x] `internal/storage/postgres/task.go` — реализация Task-репозитория через sqlc
- [x] `queries/srs.sql` + `queries/tasks.sql` — SQL-запросы
- [x] `internal/telegram/handler_task.go` — `/tasks` + callback `task:{id}:{option}`, feedback ✅/❌, итог дня
- [x] `internal/telegram/bot.go` — добавлен taskSvc, регистрация `/tasks` и callback `task:`
- [x] `cmd/makeme/main.go` — подключены srsRepo, taskRepo, генераторы, taskSvc
- [x] `internal/domain/srs/sm2_test.go` — unit-тесты SM-2: 7 кейсов
- [x] `internal/telegram/handler_task_test.go` — integration-тесты: not onboarded, no groups, generates task, full flow
- [x] `go build` и `go vet` без ошибок
- [ ] `internal/scheduler/scheduler.go` — отложен (будет в фазе 7)
- [ ] translation-задания — отложены (расширяемая архитектура через TaskGenerator)

**Проверка:** `/tasks` → бот показывает задания по одному → ответы записываются → SM-2 обновляет карточки → итог дня

---

### Фаза 5.5 — Банк слов + транскрипция + аудио [DONE]

**Цель:** пополненный банк слов (~900) с поддержкой транскрипции и произношения.

Что сделано:
- [x] `data/words.json` — ~900 слов (~150 на каждый уровень A1–C2) на базе Oxford 3000
- [x] `cmd/seed/main.go` — CLI-утилита для загрузки слов из JSON с upsert-поддержкой
- [x] `Makefile` — цель `seed-words`
- [x] `migrations/00007_words_unique_english_level.sql` — UNIQUE индекс (english, level) для upsert
- [x] `migrations/00008_add_word_transcription.sql` — колонки `transcription` и `audio_url` в `words`
- [x] `queries/srs.sql` — запрос `UpdateWordAudio`, обновлены SELECT (включают transcription, audio_url)
- [x] `internal/storage/sqlc/models.go` — поля `Transcription`, `AudioURL` в `Word`
- [x] `internal/storage/sqlc/srs.sql.go` — обновлён + добавлен `UpdateWordAudio`
- [x] `internal/domain/srs/entity.go` — поля `Transcription`, `AudioURL` в `Word`
- [x] `internal/domain/srs/repository.go` — метод `UpdateWordAudio`
- [x] `internal/storage/postgres/srs.go` — реализация `UpdateWordAudio`, маппинг новых полей
- [x] `internal/dictionary/client.go` — клиент Free Dictionary API (IPA + audio URL, без API-ключа)
- [x] `internal/domain/task/vocab.go` — транскрипция в промпте слова, audio_url в метаданных
- [x] `internal/telegram/handler_task.go` — кнопка «🔊 Послушать» + обработчик `word_audio:` callback (ленивая загрузка + кэш)
- [x] `internal/telegram/bot.go` — добавлены `srsRepo`, `dictClient`, зарегистрирован `word_audio:` callback

---

### Фаза 6 — Баллы и таблицы лидеров [DONE]

**Цель:** подсчёт баллов с нормализацией и лидерборды по группам.

Что сделано:
- [x] `migrations/00005_create_scoring.sql` — таблица `scores`
- [x] `internal/domain/scoring/entity.go` — Score, LeaderboardEntry
- [x] `internal/domain/scoring/repository.go` — интерфейс: `Upsert`, `GetLeaderboard(groupID, period)`
- [x] `internal/domain/scoring/service.go`:
  - `RecordScore(ctx, userID, groupID, date, rawPoints, level)` — применяет коэффициент, сохраняет
  - `GetLeaderboard(ctx, groupID, period)` — агрегация: day/week/month
  - Таблица коэффициентов: A1=0.6, A2=0.8, B1=1.0, B2=1.15, C1=1.3, C2=1.5
- [x] `internal/storage/postgres/scoring.go` — реализация
- [x] `queries/scoring.sql` — SQL-запросы, включая агрегацию по периодам
- [x] `internal/telegram/handler_leaderboard.go`:
  - `/leaderboard` → показать таблицу за сегодня
  - `/leaderboard week` → за неделю
  - `/leaderboard month` → за месяц
  - Формат: позиция, имя, баллы
- [x] Интеграция: при завершении задания (`handler_task.go`) → вызов `scoringService.RecordScore`
- [x] `internal/domain/task/entity.go` — добавлен `GroupID` в `Task`
- [x] `internal/storage/postgres/task.go` — `GetTaskByID` использует JOIN с `daily_sets`, убрано дублирование маппера
- [x] `internal/storage/postgres/srs.go` — исправлен маппинг поля `AudioUrl` после регенерации sqlc
- [x] `internal/domain/scoring/service_test.go` — unit-тесты: нормализация всех уровней, два вызова Upsert per RecordScore
- [x] `internal/telegram/handler_leaderboard_test.go` — integration-тесты: одна группа, несколько групп, callback, переключение day/month

**Проверка:** два пользователя в группе выполняют задания → `/leaderboard` показывает ранжированный список с нормализованными баллами.

---

### Фаза 7 — Полировка [DONE]

**Цель:** продукт готов для ежедневного использования.

Что сделано:
- [x] `/profile` — уровень, баллы за месяц, дней подряд (streak по дням с баллами), выучено слов (repetitions ≥ 3)
- [x] Graceful shutdown: дождаться завершения горутины `StartWebhook` перед выходом (`sync.WaitGroup`)
- [x] `Dockerfile` + `docker-compose.yml` (app + PostgreSQL, multi-stage build)
- [x] `.env.example` с документацией всех переменных

---

### Фаза 7.5 — Улучшения ассесмента и грамматики [DONE]

**Цель:** ассесмент доказывает пользователю почему он получил именно этот уровень; грамматические задания не повторяются.

Что сделано:
- [x] **Дедупликация** — один вопрос не повторяется в рамках одной сессии (`buildExcludeSet` + `filterExcluded`)
- [x] **Словарные вопросы** — тип вопроса «переведи слово» с адаптивной сложностью (`generateVocabQuestion`); 50% вероятность словарного вопроса, fallback на грамматику
- [x] **Итоговый экран** — после последнего вопроса: каждый вопрос, ответ пользователя, правильный ответ (`formatSummary`)
- [x] **Обоснование уровня** — объяснение «правильно X из Y на уровне B1» (`formatLevelJustification`)
- [x] **Кнопка «Не знаю»** — отдельный ряд кнопок, засчитывается как неверный в адаптивной логике, не считается ошибкой в статистике (`SkipOption = -1`)
- [x] **Грамматика в БД** — `grammar_questions` таблица (migrations 00010–00011), `GrammarRepository`, `GrammarGenerator` читает из БД
- [x] **Исправлен баг PendingQuestion** — `SubmitAnswer` использует именно тот вопрос, который был показан пользователю (хранится как `pending_question JSONB` в сессии, migration 00013)
- [x] `internal/domain/assessment/service_test.go` — unit-тесты: `TestSubmitAnswer_UsesShownQuestion`, `TestSubmitAnswer_WrongAnswerRecordedCorrectly`, `TestSubmitAnswer_SkipIsNotAnError`

**Проверка:** Ассесмент без дублей, словарные вопросы, итоговый экран с объяснениями, кнопка «Не знаю», грамматика из БД, правильная запись ответов.

---

### Фаза 8 — Проактивные сообщения (scheduler) [DONE]

**Цель:** бот сам напоминает об итогах дня и мотивирует участников.

Что сделано:
- [x] `internal/scheduler/` — пакет Scheduler с методами `Run`, `RunDailySummary`, `RunReassessReminder`
- [x] `Run()` — бесконечный цикл, стреляет каждый день в настроенное время (default 21:00 MSK)
- [x] `RunDailySummary()` — итоги дня для каждого участника хотя бы одной группы
- [x] `RunReassessReminder()` — напоминание переоценить уровень если прошло > 30 дней
- [x] `user.Repository.ListAll` + `assessment.Repository.GetLastCompleted` — новые методы
- [x] `Bot.SendMessage(ctx, chatID, text)` — проактивная отправка сообщений
- [x] Конфиг: `SCHEDULER_TIME` (default "21:00"), `TIMEZONE` (default "Europe/Moscow")
- [x] Unit-тесты: nextRunTime (4 cases), formatDailySummary, formatReassessReminder
- [x] Интеграционный тест: RunDailySummary через TestContainers + WireMock

---

### Фаза 8.5 — Соло-режим, больше заданий и улучшенный ассесмент [DONE]

**Цель:** снизить барьер входа, увеличить количество практики, сделать ассесмент более точным и честным.

Что сделано:
- [x] **Соло-режим** — `/tasks` без группы автоматически создаёт личную группу «Соло» и продолжает работу; пользователю больше не нужны друзья для старта
- [x] **5+5 заданий за раунд** — `vocabCount = 5`, `grammarCount = 5` (было 3+2); 10 заданий в одном наборе
- [x] **Несколько наборов в день** — `daily_sets.round` позволяет создавать новые наборы после завершения предыдущего; `migration 00019`
- [x] **Ассесмент 30 вопросов** — `totalQuestions = 30` (было 10); две фазы: 1–15 словарные, 16–30 грамматические
- [x] **Ассесмент стартует с A1** — адаптивный тест начинается с самого простого уровня (было B1)
- [x] **Механизм уверенности** — уровень повышается только после 2 правильных ответов подряд; `consecutive_correct` в БД (`migration 00020`)
- [x] **Сброс уровня на границе фаз** — при переходе с словарной на грамматическую фазу уровень сбрасывается до A1
- [x] **100+ слов A1 и 100+ слов A2** — seed-миграция `00021` добавляет 160+ слов A1 и 120+ слов A2 с примерами (`ON CONFLICT DO NOTHING`)
- [x] **Фазовый дисплей в боте** — «📚 Словарный запас — Вопрос 3/15» и «✍️ Грамматика — Вопрос 2/15»
- [x] **Дневной лимит раундов** — `MAX_ROUNDS_PER_DAY` (default 5) ограничивает число раундов на сутки; `task.ErrDailyLimitReached` блокирует создание нового раунда; `/tasks` показывает «🎯 Раунд N из 5 на сегодня» перед первым заданием; по достижении лимита — сообщение «возвращайся завтра»
- [x] **🤷 Не знаю для /tasks** — к каждому заданию (vocab + grammar) добавлена кнопка «🤷 Не знаю»; сервис-метод `Service.SubmitSkip` записывает результат с `skipped=true`, `correct=false`, `score=0`; миграция `00021` добавляет колонку `task_results.skipped`; для словарных заданий SRS-карточка обновляется как при неправильном ответе; фидбэк — «🤷 Пропущено. Правильный ответ: …» (+ объяснение для грамматики)

**Проверка:** `/tasks` без группы → создаётся «Соло», показывается 10 заданий; повторный `/tasks` после завершения → новый раунд; ассесмент 30 вопросов, первые 15 — слова, последние 15 — грамматика; пользователь не повышает уровень с одного правильного ответа; после 5 раундов `/tasks` показывает сообщение о лимите.

---

### Фаза 9 — Разнообразие словарных заданий (приоритет #1) [PARTIAL]

**Цель:** уйти от единственного формата «переведи слово EN→RU» — добавить несколько типов упражнений, чтобы слово прорабатывалось с разных сторон и не приедалось.

**Типы заданий (все используют уже имеющиеся данные в `words`):**

| Тип | Описание | Формат |
|-----|----------|--------|
| `vocab_en_ru` | Перевод EN → RU (уже есть) | 4 варианта, выбор |
| `vocab_ru_en` | Перевод RU → EN | 4 варианта, выбор |
| `vocab_definition` | Прочти английское определение → выбери слово | 4 варианта, выбор |
| `vocab_fill_blank` | Вставь слово в предложение (используется `example`) | 4 варианта, выбор |
| `vocab_spelling` | Услышь слово (аудио) → напечатай его | Свободный ввод |
| `vocab_context` | Прочти предложение с пропуском → напечатай слово | Свободный ввод |

**Приоритет реализации:** en→ru, ru→en, fill_blank (используют уже имеющиеся поля). definition, spelling, context — следующим шагом.

**Что нужно сделать:**
- [x] Добавить константы подтипов в `internal/domain/task/entity.go` (`VocabSubtypeEnRu`, `VocabSubtypeRuEn`, `VocabSubtypeFillBlank`) — хранятся в `Metadata["subtype"]`
- [x] `internal/domain/task/vocab.go` — рефакторинг: `buildVocabEnRu`, `buildVocabRuEn`, `buildVocabFillBlank`; `VocabGenerator` случайно выбирает тип для каждого слова
- [x] `internal/telegram/handler_task.go` — `vocabTaskLabel()` показывает разные метки для каждого подтипа
- [x] Unit-тесты: 18 тестов для трёх билдеров + 5 тестов для меток
- [ ] `vocab_definition` — определение на английском → выбрать слово
- [ ] `internal/telegram/handler_task.go` — поддержка свободного ввода для `vocab_spelling` и `vocab_context`; нечёткое сравнение
- [ ] Обновить `tasks` таблицу: CHECK constraint для новых типов (migration)
- [ ] Integration-тесты: full flow для каждого нового типа задания

**Проверка:** `/tasks` → за день встречаются разные типы словарных заданий; для `fill_blank` подсвечивается слово в примере; для `spelling` можно написать текстом; ошибки на 1 символ засчитываются как правильно.

---

### Фаза 10 — Грамматические объяснения [DONE]

**Цель:** после неправильного ответа пользователь понимает *почему* он ошибся, а не просто видит правильный вариант.

**Что нужно сделать:**
- [x] Добавить колонку `explanation TEXT` в `grammar_questions` (migration `00016`)
- [x] Заполнить объяснения по категориям (~80 категорий) в seed-миграции `00017`; формат: одно-два предложения на русском + ключевое правило
- [x] `internal/telegram/handler_task.go` — `buildIncorrectFeedback()` показывает объяснение под «❌ Неправильно. Правильный ответ: X» для грамматических заданий
- [x] Unit-тесты: 4 теста для `buildIncorrectFeedback`

**Проверка:** ответить неправильно на грамматический вопрос → под ответом появляется объяснение правила.

---

### Фаза 10.5 — Команда /learn (режим самостоятельного изучения грамматики) [DONE]

**Цель:** дать пользователю возможность проактивно изучать грамматику — выбрать тему, прочитать правило и сразу закрепить практикой, не дожидаясь ошибки в заданиях.

**Flow:**
- `/learn` → меню с inline-кнопками: темы текущего уровня + следующего (для C2 — только C2)
- Нажатие на тему → теория из `grammar_categories` (HTML-форматирование, bold заголовков) + кнопка «Начать практику»
- Практика: 3–5 вопросов по теме по одному; фидбек ✅/❌ после каждого ответа
- Практика **не влияет** на баллы и лидерборд

**Что сделано:**
- [x] `queries/grammar.sql` — два новых запроса: `GetGrammarCategoriesByLevel`, `GetGrammarQuestionsByCategory`
- [x] `make sqlc` — регенерация `internal/storage/sqlc/grammar.sql.go`
- [x] `internal/domain/task/grammar_repository.go` — новая структура `GrammarCategory`, два новых метода в интерфейсе
- [x] `internal/storage/postgres/grammar.go` — реализация `GetCategoriesByLevel`, `GetQuestionsByCategory`
- [x] `internal/telegram/handler_learn.go` — `handleLearn`, `handleLearnCallback`, хелперы `nextLevel`, парсеры callback
- [x] `internal/telegram/bot.go` — добавлены `grammarRepo` в `Bot`, зарегистрированы `/learn` и `learn:` callback
- [x] `cmd/makeme/main.go` + `testhelper_test.go` — `grammarRepo` передаётся в `telegram.New()`
- [x] `internal/telegram/handler_start.go` — `/learn` добавлен в `helpText()`
- [x] Stateless design: всё состояние кодируется в callback data (без новых таблиц в БД)
- [x] Unit-тесты: `TestNextLevel`, `TestParseLearnCat`, `TestParseLearnPractice`, `TestParseLearnQA`
- [x] Исправлен скрытый баг: `consecutive_correct` отсутствовал в SELECT-запросах ассесмента

**Проверка:** `/learn` → меню с темами двух уровней → нажать тему → теория → «Начать практику» → 3–5 вопросов → финал → «Назад к темам».

---

### Фаза 10.6 — Стратификация /learn по подтипам правил [DONE]

**Цель:** в одной сессии `/learn` гарантировать разнообразие подтипов правила (утверждение/отрицание/вопрос/spelling) и поднять лимит до 20 вопросов.

**Что сделано:**
- [x] `migrations/00022_add_grammar_subcategory.sql` — новая колонка `subcategory TEXT NOT NULL DEFAULT 'general'` + индекс `(category, subcategory)`
- [x] `queries/grammar.sql` — `subcategory` во всех SELECT и `UpsertGrammarQuestion`; убран `LIMIT` из `GetGrammarQuestionsByCategory` (стратифицированную выборку делает Go)
- [x] `make sqlc` — регенерация
- [x] `internal/domain/task/grammar_repository.go` — поле `Subcategory` в `GrammarQuestion`; метод без `limit`
- [x] `internal/domain/task/grammar_sampler.go` — `StratifiedSample` (round-robin по bucket-ам с детерминированным shuffle) + `LearnSessionSeed`
- [x] `internal/domain/task/grammar_sampler_test.go` — юнит-тесты детерминизма, равномерности, пустого входа
- [x] `internal/storage/postgres/grammar.go` — маппинг `Subcategory` и новая сигнатура `GetQuestionsByCategory`
- [x] `internal/telegram/handler_learn.go` — `learnPracticeLimit = 20`; хелпер `loadLearnSession` с seed `(user_id, category, UTC day)` гарантирует стабильный порядок вопросов между callback-ами и обновление порядка на следующий день
- [x] `cmd/seed/main.go` — парсинг и передача `subcategory` (дефолт `'general'`)
- [x] `data/grammar.json` — проставлены `subcategory` для всех 230 вопросов; объединены темы `Past Simple (regular)` + `Past Simple negatives` → `Past Simple`, `Present Simple spelling` → `Present Simple`

**Проверка:** `make seed-grammar` → `/learn` → выбрать `Past Simple` → «Начать практику» показывает до 20 вопросов с миксом affirmative/negative/interrogative.

---

### Фаза 11 — Задания на перевод фраз (RU → EN)

**Цель:** активное владение языком — самостоятельно составить предложение, а не выбрать из вариантов.

**Что нужно сделать:**
- [ ] `internal/domain/task/translation.go` — `TranslationGenerator` (заготовка уже есть в архитектуре): генерирует задания «переведи фразу с русского на английский»
- [ ] `migrations/` — таблица `translation_phrases` (level, ru_text, en_answer, alt_answers JSONB)
- [ ] `migrations/` — seed ~50 фраз на каждый уровень A1–C2
- [ ] Нечёткая проверка: нормализация (lowercase, trim пунктуации), список допустимых вариантов `alt_answers`
- [ ] `internal/telegram/handler_task.go` — поддержка свободного ввода для типа `translation`
- [ ] Integration-тест: full flow перевода

**Проверка:** `/tasks` → появляется задание «Переведи: Я хожу в школу каждый день» → пользователь пишет «I go to school every day» → засчитывается правильно.

---

### Фаза 12 — Streak-система с наградами

**Цель:** мотивировать ежедневную практику через milestone-награды — основной retention-механизм.

**Что нужно сделать:**
- [ ] `internal/domain/scoring/streaks.go` — функция `CheckStreakMilestone(streak int) (message string, bonus int, hit bool)`: возвращает поздравление и бонус для 3/7/14/30/100/365 дней
- [ ] `internal/telegram/handler_task.go` — после финального итога дня: если сегодня закрыт streak milestone — показать поздравление с бонусом
- [ ] `internal/storage/postgres/scoring.go` — `GetStreak(ctx, userID)` уже вычисляет серию; убедиться что возвращает актуальное значение
- [ ] Unit-тест: `CheckStreakMilestone` для каждого порога
- [ ] Integration-тест: пользователь выполняет задания 7 дней → получает поздравление

**Проверка:** завершить задания в 7-й день подряд → получить сообщение «🔥 7 дней подряд! +50 бонусных очков».

---

### Фаза 13 — Еженедельные и ежемесячные сводки в группе

**Цель:** планировщик отправляет групповые итоги в конце недели и месяца.

**Что нужно сделать:**
- [ ] `internal/scheduler/scheduler.go` — добавить `RunWeeklySummary()` и `RunMonthlySummary()`
- [ ] `Run()` — расширить: проверять день недели (воскресенье) и день месяца (1-е число) при ежедневном срабатывании
- [ ] `formatWeeklySummary(groupName, entries []groupSummary, period string)` — топ-3 + баллы каждого
- [ ] `formatMonthlySummary` — добавить: лучший стрик за месяц, наибольший прогресс (разница уровня или баллов)
- [ ] Unit-тесты: форматирование сводок, логика выбора дня

**Проверка:** в воскресенье в 21:00 приходит сводка недели; 1-го числа — сводка месяца.

---

### Фаза 14 — Автоматическая переоценка уровня

**Цель:** уровень адаптируется к прогрессу пользователя без ручного запуска `/assess`.

**Что нужно сделать:**
- [ ] `internal/domain/scoring/service.go` — `GetRecentAccuracy(ctx, userID, days int) (float64, error)`: доля правильных ответов за последние N дней
- [ ] `internal/scheduler/scheduler.go` — `RunLevelCheck()`: раз в неделю для каждого пользователя проверяет точность; если >80% за 14 дней → предложить пройти тест на уровень выше; если <40% за 14 дней → предложить снизить
- [ ] Предложение приходит проактивным сообщением с кнопкой «Пройти тест»
- [ ] Unit-тест: пороговая логика

**Проверка:** пользователь 2 недели делает >80% → приходит сообщение «Похоже, тебе можно попробовать B2! Хочешь пройти тест?».

---

### Фаза 15 — Расширенный банк грамматики (B2–C2)

**Цель:** продвинутые пользователи не видят повторяющихся вопросов.

**Что нужно сделать:**
- [ ] Дописать seed-миграцию: довести `grammar_questions` до ~100 вопросов для уровней B2, C1, C2
- [ ] Категории B2: Passive Voice (complex), Reported Speech, Relative Clauses, Conditionals 2+3, Wish/If only, Causative have, Emphasis (cleft sentences)
- [ ] Категории C1: Mixed conditionals, Inversion, Subjunctive, Advanced modals (needn't have, should have), Discourse markers
- [ ] Категории C2: Nominalization, Advanced passive, Ellipsis, Fronting, complex Reported Speech

**Проверка:** пользователь уровня C1 видит разные вопросы несколько дней подряд без повторов.

---

### Фаза 16 — Задания на аудирование

**Цель:** задействовать уже кэшированные аудио-файлы слов для аудирования.

**Что нужно сделать:**
- [ ] `internal/domain/task/listening.go` — `ListeningGenerator`: выбирает слова с `audio_url IS NOT NULL`; задание — отправить аудио и спросить «Какое это слово?» (4 варианта)
- [ ] `internal/telegram/handler_task.go` — отправка `audio` через Telegram API вместо текста
- [ ] Fallback: если аудио нет — пропустить слово, выбрать другое
- [ ] Integration-тест

**Проверка:** задание типа `listening` — Telegram воспроизводит аудио, пользователь выбирает правильное слово из 4 вариантов.

---

### Фаза 17 — Личные списки слов

**Цель:** пользователь добавляет свои слова (из книги, фильма), они попадают в его SRS-очередь.

**Что нужно сделать:**
- [ ] Команда `/addword <английское> <русское>` — добавить слово в таблицу `words` с `level = user.level`, привязать к пользователю (новая колонка `user_id NULLABLE` в `words` или отдельная таблица `user_words`)
- [ ] `VocabGenerator` — включать пользовательские слова в ротацию
- [ ] Команда `/mywords` — список добавленных слов с прогрессом SRS
- [ ] Integration-тест

**Проверка:** `/addword ephemeral эфемерный` → слово появляется в ежедневных заданиях в течение недели.

---

## Ключевые архитектурные решения

1. **Домен не зависит от инфраструктуры** — сервисы принимают интерфейсы репозиториев, а не конкретные реализации. Это позволяет тестировать бизнес-логику с моками, без БД.

2. **Telegram-пакет — тонкий адаптер** — никакой бизнес-логики. Только: распарсить update → вызвать домен → отформатировать ответ. Завтра можно добавить REST API или веб-клиент без дублирования логики.

3. **DailySet привязан к user+group+date** — пользователь в нескольких группах получает отдельные наборы заданий для каждой. Баллы и лидерборды скопированы по группам.

4. **Task.Prompt — строка (потенциально JSON)** — разные типы заданий хранят разную структуру. Vocab: `{"word": "ephemeral", "context": "..."}`, Grammar: `{"sentence": "She ___ to the store"}`. Схема БД стабильна, типы заданий расширяются.

5. **SM-2 — чистые функции** — алгоритм интервального повторения не зависит ни от чего. Легко тестировать, легко заменить на другой алгоритм.

---

## Проверка после каждой фазы

| Фаза | Как проверить |
|---|--------------|
| 1 | `make migrate-up` → ок, `make run` → `curl /health` → 200 |
| 2 | `/start` боту → приветствие, строка в `users` |
| 3 | Создать группу → инвайт-код → второй пользователь вступает |
| 4 | Пройти оценку → 10-15 вопросов → уровень определён |
| 5 | `/tasks` → задания → ответы → SM-2 обновление → итог дня |
| 6 | Два пользователя → задания → `/leaderboard` → ранжированный список |
| 7 | `/profile` показывает уровень/баллы/streak/слова, `docker-compose up` работает |
| 7.5 | Ассесмент без дублей, словарные вопросы, итоговый экран с объяснениями |
| 8 | Итоги дня приходят автоматически в личку участников |
| 9 | `/tasks` показывает разные типы словарных заданий (en→ru, ru→en, fill_blank, spelling) |
| 10 | Неправильный ответ на грамматику → объяснение правила |
| 10.5 | `/learn` → меню тем → теория → практика → финал |
| 11 | Задание «Переведи фразу» принимает свободный ввод |
| 12 | После 7 дней подряд → поздравление + бонус |
| 13 | В воскресенье и 1-го числа приходят сводки в группе |
| 14 | После 14 дней >80% → предложение повысить уровень |
| 15 | C1-пользователь видит разные вопросы несколько дней без повторов |
| 16 | Задание на аудирование — Telegram воспроизводит аудио, выбор из 4 вариантов |
| 17 | `/addword` → слово появляется в заданиях |

---

## Возможные доработки ассейсмента

### Найденные проблемы алгоритма

**Проблема 1: Грамматическая фаза сбрасывается на A1**

Сейчас при переходе от словарной фазы (вопросы 1–15) к грамматической (16–30) уровень принудительно сбрасывается до A1, независимо от результатов словарного теста. Пользователь с реальным B1/B2 тратит большинство грамматических вопросов на элементарном A1-уровне.

Код: `service.go`, строка ~127:
```go
if int(count) == vocabPhaseCount {
    nextLevel = user.A1
    nextConsecutive = 0
}
```

**Проблема 2: determineLevel останавливается на первом провале**

Функция `determineLevel` идёт по уровням A1→C2 и останавливается на первом уровне, где точность < 50%. Это делает итог крайне чувствительным к единственному неверному ответу на малой выборке: один неверный ответ на единственном A2-вопросе даёт A2: 0/1 = 0% и блокирует всю цепочку, игнорируя 100% по B1, B2, C1 в словарной фазе.

### Варианты исправления

**Вариант A: Начинать грамматическую фазу с уровня, выведенного из словарного блока**

Вместо сброса на A1 вычислить «словарный уровень» по итогам первых 15 вопросов и начать грамматику с него. Это уберёт скуку для сильных пользователей и сократит долю A1-вопросов.

**Вариант B: Минимальный порог выборки в determineLevel**

Не считать уровень «провальным», если на нём было меньше N вопросов (например, N=2). Один вопрос — слишком маленькая выборка для вердикта. Пропускать уровень без остановки цепочки, как сейчас пропускаются уровни без вопросов.

**Вариант C: Взвешенное голосование обеих фаз**

Итоговый уровень — максимум из «словарного уровня» и «грамматического уровня», каждый считается отдельно. Или взвешенное среднее: vocab × 0.4 + grammar × 0.6. Устраняет зависимость от единственной ошибки в одной фазе.

**Вариант D: Увеличить минимальное количество вопросов на переходных уровнях**

При адаптивном движении обеспечивать минимум 2–3 вопроса на каждом новом уровне перед вынесением оценки. Текущий алгоритм может провести на A2 ровно 1 вопрос, если пользователь сразу ошибся и откатился.

### Рекомендация

Комбинация **A + B**: грамматическую фазу начинать с уровня из словарного блока, а в `determineLevel` не останавливаться на уровнях с выборкой < 2 вопросов. Это минимальные изменения с наибольшим эффектом.
