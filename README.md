# TagOwl Backend

Go backend for the TagOwl sticker website.

It provides:
- public catalog APIs for the website frontend
- event APIs for views and favorites
- order APIs for recording purchases
- admin APIs for creating, updating, listing, and deleting stickers

The backend uses MongoDB with these defaults:
- Mongo URI: `mongodb://localhost:27017/`
- Database: `tag_owl`
- Main sticker collection: `producer`

On first startup, the server:
- connects to MongoDB
- creates indexes
- seeds the `producer` collection from [data/stickers.json](/Users/agarwalkruti/Documents/New project/tagowl/backend/data/stickers.json:1) if the collection is empty

## Prerequisites

- Go installed
- MongoDB running locally on `mongodb://localhost:27017/`

## Run The Server

Start MongoDB first, then run:

```bash
cd "/Users/agarwalkruti/Documents/New project/tagowl/backend"
go run ./cmd/api
```

The API starts on:
- `http://localhost:8080`

Health check:

```bash
curl http://localhost:8080/healthz
```

Expected response:

```json
{
  "status": "ok",
  "service": "sticker-catalog-api",
  "timestamp": "2026-04-18T12:00:00Z"
}
```

## Environment Variables

You can override the defaults like this:

```bash
cd "/Users/agarwalkruti/Documents/New project/tagowl/backend"
PORT=8081 \
MONGODB_URI="mongodb://localhost:27017/" \
MONGODB_DATABASE="tag_owl" \
MONGODB_COLLECTION="producer" \
STICKER_SEED_FILE="data/stickers.json" \
go run ./cmd/api
```

Supported env vars:

```bash
PORT=8080
MONGODB_URI=mongodb://localhost:27017/
MONGODB_DATABASE=tag_owl
MONGODB_COLLECTION=producer
STICKER_SEED_FILE=data/stickers.json
```

## API Summary

Public APIs:
- `GET /healthz`
- `GET /api/v1/home`
- `GET /api/v1/stickers`
- `GET /api/v1/stickers/{id}`

Engagement and order APIs:
- `POST /api/v1/stickers/{id}/view`
- `POST /api/v1/stickers/{id}/favorite`
- `DELETE /api/v1/stickers/{id}/favorite`
- `POST /api/v1/orders`

Admin APIs:
- `GET /api/v1/admin/stickers`
- `POST /api/v1/admin/stickers`
- `GET /api/v1/admin/stickers/{id}`
- `PATCH /api/v1/admin/stickers/{id}`
- `PATCH /api/v1/admin/stickers/{id}/price`
- `PATCH /api/v1/admin/stickers/{id}/status`
- `DELETE /api/v1/admin/stickers/{id}`

## Public APIs

### 1. Health Check

`GET /healthz`

```bash
curl http://localhost:8080/healthz
```

### 2. Homepage Data

`GET /api/v1/home`

Used by the frontend homepage to fetch:
- categories
- trending shelf
- new arrivals
- top rated
- other homepage sections returned by the backend

Query params:
- `limit`

```bash
curl "http://localhost:8080/api/v1/home?limit=4"
```

### 3. List Stickers

`GET /api/v1/stickers`

Query params:
- `category`
- `tag`
- `sort`
- `limit`

Supported `sort` values:
- `trending`
- `rank`
- `top_rated`
- `best_selling`
- `newest`
- `price_asc`
- `price_desc`

Examples:

```bash
curl "http://localhost:8080/api/v1/stickers"
```

```bash
curl "http://localhost:8080/api/v1/stickers?category=Animals&sort=trending&limit=8"
```

```bash
curl "http://localhost:8080/api/v1/stickers?tag=cute&sort=top_rated&limit=12"
```

### 4. Get Sticker By ID

`GET /api/v1/stickers/{id}`

Example:

```bash
curl "http://localhost:8080/api/v1/stickers/stk_001"
```

This returns the sticker plus live derived metrics like:
- `views7D`
- `sales7D`
- `favorites7D`
- `trendingScore`

## Engagement And Order APIs

### 5. Record A View

`POST /api/v1/stickers/{id}/view`

The backend uses `actorKey` to prevent duplicate view inflation.

```bash
curl -X POST "http://localhost:8080/api/v1/stickers/stk_001/view" \
  -H "Content-Type: application/json" \
  -d '{
    "actorKey": "session-123"
  }'
```

### 6. Add Favorite

`POST /api/v1/stickers/{id}/favorite`

```bash
curl -X POST "http://localhost:8080/api/v1/stickers/stk_001/favorite" \
  -H "Content-Type: application/json" \
  -d '{
    "actorKey": "user-123"
  }'
```

### 7. Remove Favorite

`DELETE /api/v1/stickers/{id}/favorite`

You can send `actorKey` in the query string:

```bash
curl -X DELETE "http://localhost:8080/api/v1/stickers/stk_001/favorite?actorKey=user-123"
```

### 8. Create Order

`POST /api/v1/orders`

This records sales and updates sales-related metrics.

```bash
curl -X POST "http://localhost:8080/api/v1/orders" \
  -H "Content-Type: application/json" \
  -d '{
    "customerKey": "customer-001",
    "items": [
      {
        "stickerId": "stk_001",
        "quantity": 2
      },
      {
        "stickerId": "stk_002",
        "quantity": 1
      }
    ]
  }'
```

## Admin APIs

These APIs are for your business operations.

### 9. List Admin Stickers

`GET /api/v1/admin/stickers`

By default, this returns active stickers.

```bash
curl "http://localhost:8080/api/v1/admin/stickers"
```

Include inactive and soft-deleted stickers:

```bash
curl "http://localhost:8080/api/v1/admin/stickers?includeInactive=true"
```

### 10. Create Sticker

`POST /api/v1/admin/stickers`

```bash
curl -X POST "http://localhost:8080/api/v1/admin/stickers" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "stk_admin_demo_001",
    "name": "Galaxy Cat",
    "description": "Cute galaxy-themed cat sticker",
    "imageUrl": "https://cdn.example.com/stickers/galaxy-cat.png",
    "category": "Animals",
    "tags": ["cat", "cute", "space"],
    "price": 3.99,
    "currency": "USD",
    "rank": 1,
    "rating": 4.9,
    "reviewCount": 312,
    "isNewArrival": true,
    "isActive": true
  }'
```

### 11. Get Admin Sticker By ID

`GET /api/v1/admin/stickers/{id}`

```bash
curl "http://localhost:8080/api/v1/admin/stickers/stk_admin_demo_001"
```

### 12. Update Sticker Metadata

`PATCH /api/v1/admin/stickers/{id}`

Use this for partial updates to business fields like:
- `name`
- `description`
- `imageUrl`
- `category`
- `tags`
- `price`
- `currency`
- `rank`
- `rating`
- `reviewCount`
- `isNewArrival`
- `isActive`

```bash
curl -X PATCH "http://localhost:8080/api/v1/admin/stickers/stk_admin_demo_001" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Galaxy Cat Deluxe",
    "description": "Updated admin description",
    "tags": ["cat", "cute", "space", "premium"],
    "rank": 2
  }'
```

### 13. Update Price Only

`PATCH /api/v1/admin/stickers/{id}/price`

```bash
curl -X PATCH "http://localhost:8080/api/v1/admin/stickers/stk_admin_demo_001/price" \
  -H "Content-Type: application/json" \
  -d '{
    "price": 4.49,
    "currency": "USD"
  }'
```

### 14. Update Status

`PATCH /api/v1/admin/stickers/{id}/status`

Set `isActive` to `false` to hide a sticker from the public APIs.

```bash
curl -X PATCH "http://localhost:8080/api/v1/admin/stickers/stk_admin_demo_001/status" \
  -H "Content-Type: application/json" \
  -d '{
    "isActive": false
  }'
```

### 15. Soft Delete Sticker

`DELETE /api/v1/admin/stickers/{id}`

This is a soft delete. The sticker is marked inactive instead of being permanently removed from MongoDB.

```bash
curl -X DELETE "http://localhost:8080/api/v1/admin/stickers/stk_admin_demo_001"
```

## Sticker Schema

Stored sticker document:

```json
{
  "id": "stk_001",
  "name": "Galaxy Cat",
  "description": "Cute galaxy-themed cat sticker",
  "imageUrl": "https://cdn.example.com/stickers/galaxy-cat.png",
  "category": "Animals",
  "tags": ["cat", "cute", "space"],
  "price": 3.99,
  "currency": "USD",
  "rank": 1,
  "rating": 4.9,
  "reviewCount": 312,
  "isNewArrival": true,
  "isActive": true,
  "createdAt": "2026-04-10T09:00:00Z",
  "updatedAt": "2026-04-10T09:00:00Z"
}
```

Derived response fields:

```json
{
  "views7D": 1860,
  "sales7D": 41,
  "favorites7D": 128,
  "trendingScore": 1088.6
}
```

## MongoDB Collections

Main collection:
- `producer`

Supporting collections:
- `producer_daily_metrics`
- `producer_view_events`
- `producer_favorites`
- `producer_orders`

## Metrics And Trending

The backend records source events and builds rolling metrics:
- `views7D`
- `favorites7D`
- `sales7D`

Trending uses recent business signals instead of only category:

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
- `category` is mainly for browsing and filtering
- `tags` are for exact-match filtering
- `rank` acts as an editorial boost and tie-breaker
- homepage trending can mix categories instead of showing only one category

## Notes

- Public APIs only return active stickers.
- Admin APIs can return inactive stickers.
- In MongoDB, your business sticker ID is stored in the `id` field, while Mongo also creates its own `_id`.
- When searching in MongoDB Compass or `mongosh`, search with:

```json
{ "id": "stk_001" }
```
