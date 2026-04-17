# Sticker Backend

This backend is written in Go and now uses SQLite as the main data store.

On first run it:
- creates `data/stickers.db`
- creates the SQLite tables
- seeds the catalog from `data/stickers.json`

## Run

```bash
cd /Users/agarwalkruti/Documents/New\ project/tagowl/backend
go run ./cmd/api
```

Default port: `8080`

Optional env vars:

```bash
PORT=8080
STICKER_DB_FILE=data/stickers.db
STICKER_SEED_FILE=data/stickers.json
```

## Main Read APIs

1. `GET /api/v1/home`

Returns the homepage payload in one request.

Query params:
- `limit`

2. `GET /api/v1/stickers`

Returns sticker cards for browse and search pages.

Query params:
- `category`
- `tag`
- `sort` with values `trending`, `rank`, `top_rated`, `best_selling`, `newest`, `price_asc`, `price_desc`
- `limit`

3. `GET /api/v1/stickers/{id}`

Returns one sticker with live 7-day metrics and computed `trendingScore`.

## Metric Recording APIs

4. `POST /api/v1/stickers/{id}/view`

Example body:

```json
{
  "actorKey": "session-123"
}
```

Behavior:
- records one view for that sticker for the current day
- if the same `actorKey` sends another view on the same day, it is ignored
- if no `actorKey` is sent, the backend falls back to request IP

5. `POST /api/v1/stickers/{id}/favorite`

Example body:

```json
{
  "actorKey": "session-123"
}
```

Behavior:
- creates or re-activates a favorite for that actor
- increments `favorites_7d` only when the favorite becomes active again

6. `DELETE /api/v1/stickers/{id}/favorite?actorKey=session-123`

Behavior:
- marks the favorite inactive
- does not reduce `favorites_7d`, because that metric tracks favorite actions added in the last 7 days

7. `POST /api/v1/orders`

Example body:

```json
{
  "customerKey": "customer-123",
  "items": [
    {
      "stickerId": "stk_001",
      "quantity": 2
    }
  ]
}
```

Behavior:
- creates an order row
- creates order item rows
- increments `sales_7d` by the ordered quantity

## Sticker Schema

Stored sticker metadata:

```json
{
  "id": "stk_001",
  "name": "Galaxy Cat",
  "imageUrl": "https://cdn.example.com/stickers/galaxy-cat.png",
  "category": "Animals",
  "tags": ["cat", "cute", "space"],
  "price": 3.99,
  "currency": "USD",
  "rank": 1,
  "rating": 4.9,
  "reviewCount": 312,
  "isNewArrival": true,
  "createdAt": "2026-04-10T09:00:00Z"
}
```

Derived metrics returned by the API:

```json
{
  "views7D": 1861,
  "sales7D": 43,
  "favorites7D": 129,
  "trendingScore": 1111.7
}
```

## SQLite Tables

- `stickers`
- `sticker_tags`
- `sticker_daily_metrics`
- `sticker_view_events`
- `sticker_favorites`
- `orders`
- `order_items`

## How The 7-Day Metrics Work

- `views7D` comes from summing `views_count` in `sticker_daily_metrics` for the last 7 days
- `favorites7D` comes from summing `favorites_count` in `sticker_daily_metrics` for the last 7 days
- `sales7D` comes from summing `sales_count` in `sticker_daily_metrics` for the last 7 days

That means the API does not trust the frontend to send final metric values.
The frontend only tells the backend about events, and the backend updates the SQLite tables.

## Trending Logic

Current formula:

```text
trending_score =
  (sales_7d * 10) +
  (favorites_7d * 3) +
  (views_7d * 0.12) +
  (rating * 4) +
  (min(review_count, 200) * 0.1) +
  freshness_boost +
  editorial_boost
```

Notes:
- `category` is for browse/filtering
- `tags` are for exact tag filtering
- `rank` is now an editorial boost and tie-breaker
- `Top Trending` mixes categories and applies a category cap so one category does not take over the whole shelf

## Concurrency Notes

- HTTP requests are handled concurrently by Go’s server
- SQLite is used in WAL mode with a busy timeout
- writes are done transactionally so views, favorites, and sales update safely
- this is a good starter setup for a small product; for heavy write traffic, PostgreSQL is still the next step
