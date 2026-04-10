# makeMe — План архитектуры

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
    Port          string // "8080"
    DatabaseURL   string // "postgres://user:pass@host:5432/makeme"
    TelegramToken string
    WebhookURL    string // "https://domain.com/webhook/{token}"
    LogLevel      string // "debug" / "info" / "warn" / "error"
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

### Фаза 8 — Проактивные сообщения (scheduler)

**Цель:** бот сам напоминает об итогах дня и мотивирует участников.

Что делаем (отложено — требует отдельного дизайна):
- Scheduler: бот присылает итоги дня/месяца участникам в личку
- Выбор канала: личка каждого участника (не групповой чат — бот работает в private mode)
- Время отправки: настраиваемое через конфиг (по умолчанию 21:00 по московскому времени)
- Переассесмент: повторная оценка уровня с cooldown (не чаще раза в 30 дней)

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
