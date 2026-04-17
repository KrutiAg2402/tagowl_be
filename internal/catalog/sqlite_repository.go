package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrStickerNotFound  = errors.New("sticker not found")
	ErrActorKeyRequired = errors.New("actor key is required")
	ErrEmptyOrder       = errors.New("order requires at least one item")
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS stickers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  image_url TEXT NOT NULL,
  category TEXT NOT NULL,
  price REAL NOT NULL,
  currency TEXT NOT NULL,
  rank INTEGER NOT NULL,
  rating REAL NOT NULL,
  review_count INTEGER NOT NULL DEFAULT 0,
  is_new_arrival INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sticker_tags (
  sticker_id TEXT NOT NULL,
  tag TEXT NOT NULL,
  PRIMARY KEY (sticker_id, tag),
  FOREIGN KEY (sticker_id) REFERENCES stickers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sticker_daily_metrics (
  sticker_id TEXT NOT NULL,
  metric_date TEXT NOT NULL,
  views_count INTEGER NOT NULL DEFAULT 0,
  favorites_count INTEGER NOT NULL DEFAULT 0,
  sales_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (sticker_id, metric_date),
  FOREIGN KEY (sticker_id) REFERENCES stickers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sticker_view_events (
  sticker_id TEXT NOT NULL,
  actor_key TEXT NOT NULL,
  viewed_on TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (sticker_id, actor_key, viewed_on),
  FOREIGN KEY (sticker_id) REFERENCES stickers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sticker_favorites (
  sticker_id TEXT NOT NULL,
  actor_key TEXT NOT NULL,
  is_active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (sticker_id, actor_key),
  FOREIGN KEY (sticker_id) REFERENCES stickers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS orders (
  id TEXT PRIMARY KEY,
  customer_key TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS order_items (
  order_id TEXT NOT NULL,
  sticker_id TEXT NOT NULL,
  quantity INTEGER NOT NULL,
  PRIMARY KEY (order_id, sticker_id),
  FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
  FOREIGN KEY (sticker_id) REFERENCES stickers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_stickers_category ON stickers(category);
CREATE INDEX IF NOT EXISTS idx_sticker_tags_tag ON sticker_tags(tag);
CREATE INDEX IF NOT EXISTS idx_sticker_daily_metrics_date ON sticker_daily_metrics(metric_date);
CREATE INDEX IF NOT EXISTS idx_sticker_daily_metrics_sticker_date ON sticker_daily_metrics(sticker_id, metric_date);
CREATE INDEX IF NOT EXISTS idx_sticker_view_events_actor ON sticker_view_events(actor_key, viewed_on);
CREATE INDEX IF NOT EXISTS idx_sticker_favorites_actor ON sticker_favorites(actor_key, is_active);
CREATE INDEX IF NOT EXISTS idx_order_items_sticker ON order_items(sticker_id);
`

const stickerSelectBase = `
WITH metrics AS (
  SELECT
    sticker_id,
    SUM(views_count) AS views_7d,
    SUM(favorites_count) AS favorites_7d,
    SUM(sales_count) AS sales_7d
  FROM sticker_daily_metrics
  WHERE metric_date >= ?
  GROUP BY sticker_id
)
SELECT
  s.id,
  s.name,
  s.image_url,
  s.category,
  COALESCE(GROUP_CONCAT(t.tag, '|'), '') AS tags_csv,
  s.price,
  s.currency,
  s.rank,
  s.rating,
  s.review_count,
  s.is_new_arrival,
  COALESCE(metrics.views_7d, 0) AS views_7d,
  COALESCE(metrics.sales_7d, 0) AS sales_7d,
  COALESCE(metrics.favorites_7d, 0) AS favorites_7d,
  s.created_at
FROM stickers s
LEFT JOIN sticker_tags t ON t.sticker_id = s.id
LEFT JOIN metrics ON metrics.sticker_id = s.id
`

type SQLiteRepository struct {
	db      *sql.DB
	writeMu sync.Mutex
}

type catalogFile struct {
	Stickers []Sticker `json:"stickers"`
}

func NewSQLiteRepository(dbPath, seedPath string) (*SQLiteRepository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", buildSQLiteDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply sqlite schema: %w", err)
	}

	repo := &SQLiteRepository{db: db}
	if err := repo.seedIfEmpty(context.Background(), seedPath); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) List(ctx context.Context, filter StickerFilter) ([]Sticker, error) {
	normalized := normalizeFilter(filter)
	items, err := r.queryStickers(ctx, normalized.Category, normalized.Tag)
	if err != nil {
		return nil, err
	}

	sortStickers(items, normalized.Sort)
	if len(items) > normalized.Limit {
		items = items[:normalized.Limit]
	}

	return items, nil
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id string) (Sticker, bool, error) {
	sticker, err := r.queryStickerByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Sticker{}, false, nil
		}
		return Sticker{}, false, err
	}

	return sticker, true, nil
}

func (r *SQLiteRepository) Home(ctx context.Context, limit int) (HomeResponse, error) {
	items, err := r.queryStickers(ctx, "", "")
	if err != nil {
		return HomeResponse{}, err
	}
	if len(items) == 0 {
		return HomeResponse{}, errors.New("catalog is empty")
	}

	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = defaultHomeLimit
	}
	if normalizedLimit > maxListLimit {
		normalizedLimit = maxListLimit
	}

	categoriesSet := make(map[string]struct{})
	for _, sticker := range items {
		categoriesSet[sticker.Category] = struct{}{}
	}

	categories := make([]string, 0, len(categoriesSet))
	for category := range categoriesSet {
		categories = append(categories, category)
	}
	sortStrings(categories)

	topTrending := cloneStickers(items)
	sortStickers(topTrending, "trending")
	topTrending = limitWithCategoryDiversity(topTrending, normalizedLimit, trendingCategoryCap)

	newArrivals := cloneStickers(items)
	sortStickers(newArrivals, "newest")
	if len(newArrivals) > normalizedLimit {
		newArrivals = newArrivals[:normalizedLimit]
	}

	topRated := cloneStickers(items)
	sortStickers(topRated, "top_rated")
	if len(topRated) > normalizedLimit {
		topRated = topRated[:normalizedLimit]
	}

	return HomeResponse{
		Categories: categories,
		Sections: []HomeSection{
			{Key: "top-trending", Title: "Top Trending", Stickers: topTrending},
			{Key: "new-arrivals", Title: "New Arrivals", Stickers: newArrivals},
			{Key: "top-rated", Title: "Top Rated", Stickers: topRated},
		},
	}, nil
}

func (r *SQLiteRepository) RecordView(ctx context.Context, stickerID, actorKey string) (EventResponse, error) {
	now := time.Now().UTC()
	metricDate := now.Format("2006-01-02")

	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return EventResponse{}, err
	}
	defer tx.Rollback()

	if err := ensureStickerExistsTx(ctx, tx, stickerID); err != nil {
		return EventResponse{}, err
	}

	recorded := false
	if strings.TrimSpace(actorKey) == "" {
		if err := upsertDailyMetricTx(ctx, tx, stickerID, metricDate, 1, 0, 0); err != nil {
			return EventResponse{}, err
		}
		recorded = true
	} else {
		result, err := tx.ExecContext(
			ctx,
			`INSERT OR IGNORE INTO sticker_view_events (sticker_id, actor_key, viewed_on, created_at) VALUES (?, ?, ?, ?)`,
			stickerID,
			actorKey,
			metricDate,
			now.Format(time.RFC3339),
		)
		if err != nil {
			return EventResponse{}, err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return EventResponse{}, err
		}
		recorded = rowsAffected > 0
		if recorded {
			if err := upsertDailyMetricTx(ctx, tx, stickerID, metricDate, 1, 0, 0); err != nil {
				return EventResponse{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return EventResponse{}, err
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil {
		return EventResponse{}, err
	}
	if !ok {
		return EventResponse{}, ErrStickerNotFound
	}

	action := "view_recorded"
	if !recorded {
		action = "view_already_recorded_today"
	}

	return EventResponse{
		Action:   action,
		Recorded: recorded,
		Sticker:  sticker,
	}, nil
}

func (r *SQLiteRepository) AddFavorite(ctx context.Context, stickerID, actorKey string) (EventResponse, error) {
	actorKey = strings.TrimSpace(actorKey)
	if actorKey == "" {
		return EventResponse{}, ErrActorKeyRequired
	}

	now := time.Now().UTC()
	metricDate := now.Format("2006-01-02")

	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return EventResponse{}, err
	}
	defer tx.Rollback()

	if err := ensureStickerExistsTx(ctx, tx, stickerID); err != nil {
		return EventResponse{}, err
	}

	var active int
	err = tx.QueryRowContext(
		ctx,
		`SELECT is_active FROM sticker_favorites WHERE sticker_id = ? AND actor_key = ?`,
		stickerID,
		actorKey,
	).Scan(&active)

	recorded := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO sticker_favorites (sticker_id, actor_key, is_active, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
			stickerID,
			actorKey,
			now.Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
		if err != nil {
			return EventResponse{}, err
		}
		recorded = true
	case err != nil:
		return EventResponse{}, err
	case active == 0:
		_, err = tx.ExecContext(
			ctx,
			`UPDATE sticker_favorites SET is_active = 1, updated_at = ? WHERE sticker_id = ? AND actor_key = ?`,
			now.Format(time.RFC3339),
			stickerID,
			actorKey,
		)
		if err != nil {
			return EventResponse{}, err
		}
		recorded = true
	default:
		recorded = false
	}

	if recorded {
		if err := upsertDailyMetricTx(ctx, tx, stickerID, metricDate, 0, 1, 0); err != nil {
			return EventResponse{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return EventResponse{}, err
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil {
		return EventResponse{}, err
	}
	if !ok {
		return EventResponse{}, ErrStickerNotFound
	}

	action := "favorite_created"
	if !recorded {
		action = "favorite_already_active"
	}

	return EventResponse{
		Action:   action,
		Recorded: recorded,
		Sticker:  sticker,
	}, nil
}

func (r *SQLiteRepository) RemoveFavorite(ctx context.Context, stickerID, actorKey string) (EventResponse, error) {
	actorKey = strings.TrimSpace(actorKey)
	if actorKey == "" {
		return EventResponse{}, ErrActorKeyRequired
	}

	now := time.Now().UTC()

	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return EventResponse{}, err
	}
	defer tx.Rollback()

	if err := ensureStickerExistsTx(ctx, tx, stickerID); err != nil {
		return EventResponse{}, err
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE sticker_favorites SET is_active = 0, updated_at = ? WHERE sticker_id = ? AND actor_key = ? AND is_active = 1`,
		now.Format(time.RFC3339),
		stickerID,
		actorKey,
	)
	if err != nil {
		return EventResponse{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return EventResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return EventResponse{}, err
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil {
		return EventResponse{}, err
	}
	if !ok {
		return EventResponse{}, ErrStickerNotFound
	}

	action := "favorite_removed"
	recorded := rowsAffected > 0
	if !recorded {
		action = "favorite_not_active"
	}

	return EventResponse{
		Action:   action,
		Recorded: recorded,
		Sticker:  sticker,
	}, nil
}

func (r *SQLiteRepository) CreateOrder(ctx context.Context, request OrderCreateRequest) (OrderResponse, error) {
	items, totalQuantity, err := normalizeOrderItems(request.Items)
	if err != nil {
		return OrderResponse{}, err
	}

	now := time.Now().UTC()
	metricDate := now.Format("2006-01-02")
	orderID := uuid.NewString()

	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return OrderResponse{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO orders (id, customer_key, created_at) VALUES (?, ?, ?)`,
		orderID,
		strings.TrimSpace(request.CustomerKey),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return OrderResponse{}, err
	}

	stickerIDs := make([]string, 0, len(items))
	for _, item := range items {
		if err := ensureStickerExistsTx(ctx, tx, item.StickerID); err != nil {
			return OrderResponse{}, err
		}

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO order_items (order_id, sticker_id, quantity) VALUES (?, ?, ?)`,
			orderID,
			item.StickerID,
			item.Quantity,
		)
		if err != nil {
			return OrderResponse{}, err
		}

		if err := upsertDailyMetricTx(ctx, tx, item.StickerID, metricDate, 0, 0, item.Quantity); err != nil {
			return OrderResponse{}, err
		}

		stickerIDs = append(stickerIDs, item.StickerID)
	}

	if err := tx.Commit(); err != nil {
		return OrderResponse{}, err
	}

	stickers := make([]Sticker, 0, len(stickerIDs))
	for _, stickerID := range stickerIDs {
		sticker, ok, err := r.GetByID(ctx, stickerID)
		if err != nil {
			return OrderResponse{}, err
		}
		if ok {
			stickers = append(stickers, sticker)
		}
	}

	return OrderResponse{
		OrderID:       orderID,
		CustomerKey:   strings.TrimSpace(request.CustomerKey),
		ItemCount:     len(items),
		TotalQuantity: totalQuantity,
		CreatedAt:     now,
		Stickers:      stickers,
	}, nil
}

func (r *SQLiteRepository) seedIfEmpty(ctx context.Context, seedPath string) error {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stickers`).Scan(&count); err != nil {
		return fmt.Errorf("count stickers: %w", err)
	}
	if count > 0 {
		return nil
	}

	payload, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}

	var file catalogFile
	if err := json.Unmarshal(payload, &file); err != nil {
		return fmt.Errorf("decode seed file: %w", err)
	}

	if len(file.Stickers) == 0 {
		return errors.New("seed file contains no stickers")
	}

	now := time.Now().UTC().Format("2006-01-02")
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, sticker := range file.Stickers {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO stickers (id, name, image_url, category, price, currency, rank, rating, review_count, is_new_arrival, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sticker.ID,
			sticker.Name,
			sticker.ImageURL,
			sticker.Category,
			sticker.Price,
			sticker.Currency,
			sticker.Rank,
			sticker.Rating,
			sticker.ReviewCount,
			boolToInt(sticker.IsNewArrival),
			sticker.CreatedAt.UTC().Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("seed stickers: %w", err)
		}

		for _, tag := range sticker.Tags {
			_, err = tx.ExecContext(
				ctx,
				`INSERT INTO sticker_tags (sticker_id, tag) VALUES (?, ?)`,
				sticker.ID,
				tag,
			)
			if err != nil {
				return fmt.Errorf("seed tags: %w", err)
			}
		}

		if err := upsertDailyMetricTx(ctx, tx, sticker.ID, now, sticker.Views7D, sticker.Favorites7D, sticker.Sales7D); err != nil {
			return fmt.Errorf("seed metrics: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed data: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) queryStickers(ctx context.Context, category, tag string) ([]Sticker, error) {
	sevenDayStart := time.Now().UTC().AddDate(0, 0, -6).Format("2006-01-02")
	rows, err := r.db.QueryContext(
		ctx,
		stickerSelectBase+`
WHERE (? = '' OR LOWER(s.category) = LOWER(?))
  AND (? = '' OR EXISTS (
    SELECT 1
    FROM sticker_tags ft
    WHERE ft.sticker_id = s.id
      AND LOWER(ft.tag) = LOWER(?)
  ))
GROUP BY
  s.id, s.name, s.image_url, s.category, s.price, s.currency, s.rank, s.rating,
  s.review_count, s.is_new_arrival, s.created_at, metrics.views_7d, metrics.sales_7d, metrics.favorites_7d
`,
		sevenDayStart,
		category, category,
		tag, tag,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanStickerRows(rows)
}

func (r *SQLiteRepository) queryStickerByID(ctx context.Context, id string) (Sticker, error) {
	sevenDayStart := time.Now().UTC().AddDate(0, 0, -6).Format("2006-01-02")
	row := r.db.QueryRowContext(
		ctx,
		stickerSelectBase+`
WHERE s.id = ?
GROUP BY
  s.id, s.name, s.image_url, s.category, s.price, s.currency, s.rank, s.rating,
  s.review_count, s.is_new_arrival, s.created_at, metrics.views_7d, metrics.sales_7d, metrics.favorites_7d
`,
		sevenDayStart,
		id,
	)

	sticker, err := scanStickerRow(row)
	if err != nil {
		return Sticker{}, err
	}

	return enrichSticker(sticker, time.Now().UTC()), nil
}

func scanStickerRows(rows *sql.Rows) ([]Sticker, error) {
	now := time.Now().UTC()
	items := make([]Sticker, 0)
	for rows.Next() {
		sticker, err := scanStickerRow(rows)
		if err != nil {
			return nil, err
		}
		sticker = enrichSticker(sticker, now)
		items = append(items, sticker)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanStickerRow(row scanner) (Sticker, error) {
	var (
		sticker       Sticker
		tagsCSV       string
		createdAtText string
		isNewArrival  int
	)

	if err := row.Scan(
		&sticker.ID,
		&sticker.Name,
		&sticker.ImageURL,
		&sticker.Category,
		&tagsCSV,
		&sticker.Price,
		&sticker.Currency,
		&sticker.Rank,
		&sticker.Rating,
		&sticker.ReviewCount,
		&isNewArrival,
		&sticker.Views7D,
		&sticker.Sales7D,
		&sticker.Favorites7D,
		&createdAtText,
	); err != nil {
		return Sticker{}, err
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtText)
	if err != nil {
		return Sticker{}, err
	}

	sticker.Tags = splitTags(tagsCSV)
	sticker.IsNewArrival = isNewArrival == 1
	sticker.CreatedAt = createdAt

	return sticker, nil
}

func ensureStickerExistsTx(ctx context.Context, tx *sql.Tx, stickerID string) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM stickers WHERE id = ?`, stickerID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrStickerNotFound
		}
		return err
	}
	return nil
}

func upsertDailyMetricTx(ctx context.Context, tx *sql.Tx, stickerID, metricDate string, viewsDelta, favoritesDelta, salesDelta int) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO sticker_daily_metrics (sticker_id, metric_date, views_count, favorites_count, sales_count)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(sticker_id, metric_date) DO UPDATE SET
		   views_count = views_count + excluded.views_count,
		   favorites_count = favorites_count + excluded.favorites_count,
		   sales_count = sales_count + excluded.sales_count`,
		stickerID,
		metricDate,
		viewsDelta,
		favoritesDelta,
		salesDelta,
	)
	return err
}

func normalizeOrderItems(items []OrderItemRequest) ([]OrderItemRequest, int, error) {
	if len(items) == 0 {
		return nil, 0, ErrEmptyOrder
	}

	merged := make(map[string]int)
	totalQuantity := 0
	for _, item := range items {
		stickerID := strings.TrimSpace(item.StickerID)
		if stickerID == "" {
			return nil, 0, errors.New("order item requires stickerId")
		}
		if item.Quantity <= 0 {
			return nil, 0, errors.New("order item quantity must be greater than zero")
		}
		merged[stickerID] += item.Quantity
		totalQuantity += item.Quantity
	}

	normalized := make([]OrderItemRequest, 0, len(merged))
	for stickerID, quantity := range merged {
		normalized = append(normalized, OrderItemRequest{
			StickerID: stickerID,
			Quantity:  quantity,
		})
	}

	sortOrderItems(normalized)
	return normalized, totalQuantity, nil
}

func sortOrderItems(items []OrderItemRequest) {
	if len(items) < 2 {
		return
	}

	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].StickerID < items[i].StickerID {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func buildSQLiteDSN(dbPath string) string {
	absolutePath, err := filepath.Abs(dbPath)
	if err == nil {
		dbPath = absolutePath
	}

	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "foreign_keys(ON)")
	query.Add("_pragma", "synchronous(NORMAL)")

	return (&url.URL{
		Scheme:   "file",
		Path:     dbPath,
		RawQuery: query.Encode(),
	}).String()
}

func splitTags(tagsCSV string) []string {
	if tagsCSV == "" {
		return []string{}
	}

	parts := strings.Split(tagsCSV, "|")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tags = append(tags, part)
		}
	}

	return tags
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func sortStrings(items []string) {
	if len(items) < 2 {
		return
	}

	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
