# ТЗ: CDN File Service на базе SAI Service Framework

---

## 1. Обзор системы

**Цель:** Распределённый CDN-сервис для хранения, раздачи и управления файлами с геораспределением нод, защитой от атак и автоматическим выбором ближайшего сервера.

**Стек:**
- Go + SAI Service Framework (FastHTTP)
- MinIO / S3-совместимое хранилище (бэкенд)
- Redis (метаданные, сессии, rate limiting, hot cache)
- MongoDB (метаданные файлов, аналитика)
- Consul / etcd (service discovery, конфигурация нод)
- Prometheus + Grafana (метрики)

---

## 2. Архитектура

```
Client
  │
  ▼
[GeoDNS / Anycast IP]
  │
  ├─► [Edge Node EU] ──┐
  ├─► [Edge Node US] ──┼──► [Shield Node] ──► [Origin Cluster]
  └─► [Edge Node AS] ──┘                           │
                                               [MinIO / S3]
                                               [MongoDB]
                                               [Redis Cluster]
```

### Типы нод

| Тип | Роль |
|-----|------|
| **Origin** | Мастер-хранилище, приём загрузок, управление метаданными |
| **Shield** | Origin Shielding — промежуточный буфер перед origin, снижает нагрузку |
| **Edge** | Кэширующие ноды, раздача файлов, ближе к пользователям |
| **CDN Controller** | Оркестрация нод, инвалидация кэша, балансировка |

---

## 3. Микросервисы

### 3.1 sai-cdn-origin

Приём, валидация и хранение файлов.

```
cmd/main.go
internal/
  handlers/
    upload.go       # загрузка файлов
    manage.go       # управление (delete, update meta)
  service/
    upload.go       # валидация, антивирус, оптимизация
    storage.go      # работа с MinIO/S3
  repository/
    file.go         # CRUD метаданных в MongoDB
types/
  request.go
  response.go
  config.go
```

**Эндпоинты:**
```
POST   /api/v1/files/upload          # загрузка файла
POST   /api/v1/files/upload/chunk    # chunked upload
POST   /api/v1/files/upload/init     # инициализация multipart
POST   /api/v1/files/upload/complete # завершение multipart
DELETE /api/v1/files/:id             # удаление
PUT    /api/v1/files/:id/meta        # обновление метаданных
GET    /api/v1/files/:id/info        # информация о файле
POST   /api/v1/files/invalidate      # инвалидация кэша на edge нодах
```

### 3.2 sai-cdn-edge

Кэширование и раздача файлов с минимальной задержкой.

```
cmd/main.go
internal/
  handlers/
    serve.go        # отдача файлов
    proxy.go        # проксирование на origin при cache miss
  service/
    cache.go        # управление локальным кэшем
    prefetch.go     # предзагрузка популярных файлов
  middleware/
    ratelimit.go    # rate limiting
    hotlink.go      # защита от hotlinking
types/
```

**Эндпоинты:**
```
GET  /files/:id              # отдача файла (основной)
GET  /files/:id/thumb/:size  # ресайз изображений на лету
HEAD /files/:id              # проверка существования
```

### 3.3 sai-cdn-controller

Управление топологией нод, маршрутизация, мониторинг.

```
cmd/main.go
internal/
  handlers/
    nodes.go        # управление нодами
    routing.go      # правила маршрутизации
    analytics.go    # статистика
  service/
    discovery.go    # Consul/etcd интеграция
    selector.go     # выбор ближайшей ноды
    invalidation.go # инвалидация кэша по всем нодам
types/
```

**Эндпоинты:**
```
GET  /api/v1/nodes           # список нод
POST /api/v1/nodes/register  # регистрация ноды
PUT  /api/v1/nodes/:id       # обновление состояния ноды
GET  /api/v1/route?ip=...    # выбор оптимальной ноды для IP
POST /api/v1/invalidate      # инвалидация файла на всех нодах
GET  /api/v1/stats           # агрегированная аналитика
```

---

## 4. Выбор ближайшей ноды

### 4.1 Алгоритм выбора

```
1. GeoDNS → регион (EU/US/AS/...)
2. Получить список нод региона из Consul
3. Фильтр по health status (исключить unhealthy)
4. Скоринг ноды:
   score = w1*(1/latency) + w2*(1/load) + w3*(cache_hit_rate) + w4*(bandwidth)
5. Выбрать ноду с max score
6. Fallback: соседний регион → origin
```

### 4.2 Метрики ноды для скоринга

```go
type NodeMetrics struct {
    Latency       float64 // RTT в мс (ping от controller)
    CPULoad       float64 // 0.0 - 1.0
    BandwidthFree int64   // свободная полоса Mbps
    CacheHitRate  float64 // % попаданий в кэш
    ActiveConns   int64   // текущие соединения
    DiskFree      int64   // свободное место GB
}
```

### 4.3 Health Check

Каждая нода репортит в Consul каждые 10 секунд:
- HTTP GET `/health` → 200 OK
- Метрики в формате Prometheus

Controller помечает ноду unhealthy при:
- 3 подряд неудачных проверках
- Latency > 500ms
- CPULoad > 0.9
- DiskFree < 5GB

### 4.4 Клиентский редирект

```
GET /files/:id
→ 302 Location: https://edge-eu-1.cdn.example.com/files/:id
   (или 307 при наличии signed URL)
```

Альтернатива: прозрачное проксирование без редиректа (выше нагрузка на controller).

---

## 5. Загрузка файлов

### 5.1 Обычная загрузка (до 100MB)

```
POST /api/v1/files/upload
Content-Type: multipart/form-data

file=<binary>
meta={"name":"photo.jpg","tags":["photo"],"public":true}
```

### 5.2 Chunked upload (более 100MB)

```
1. POST /upload/init        → upload_id
2. POST /upload/chunk       × N (параллельно, до 8 потоков)
   upload_id, chunk_index, data
3. POST /upload/complete    → file_id
   upload_id, chunks_count, checksum
```

Состояние чанков хранится в Redis с TTL 24h.

### 5.3 Валидация при загрузке

- Проверка MIME-типа по magic bytes (не по расширению)
- Максимальный размер файла из конфига
- Whitelist разрешённых типов
- Проверка имени файла (path traversal)
- ClamAV интеграция для антивирусной проверки (async)
- Лимит загрузок на пользователя (rate limiting по user_id)

### 5.4 Постобработка (async через Action System)

```go
sai.Actions().Subscribe("file.uploaded", func(msg *types.ActionMessage) error {
    // генерация превью изображений
    // извлечение метаданных (EXIF, длительность видео)
    // антивирусная проверка
    // репликация на edge ноды (prefetch)
    return nil
})
```

---

## 6. Кэширование на Edge нодах

### 6.1 Стратегия

- **L1:** In-memory LRU кэш (горячие файлы, < 10MB, до 2GB RAM)
- **L2:** Локальный диск SSD (до 500GB)
- **L3:** Shield нода (Origin Shielding)
- **L4:** Origin (MinIO/S3)

### 6.2 Perma-Cache

Постоянный слой кэша на Shield ноде — файлы никогда не вылетают при TTL-miss на edge.

```
Edge L1/L2 miss
  → Shield (Perma-Cache, всегда hit если файл существует)
  → Origin (только при первом запросе файла)
```

- Хранилище Shield: MinIO / локальный SSD большого объёма
- Инвалидация: только явная (через API), не по TTL
- Экономия нагрузки на origin: до 95% запросов останавливаются на Shield

### 6.3 Request Coalescing

Если N запросов к одному некэшированному файлу приходят одновременно — к origin идёт **один** запрос, остальные ждут результата.

```go
type CoalescingGroup struct {
    mu      sync.Mutex
    waiters []chan struct{}
    done    bool
    err     error
}
// singleflight.Group из stdlib — готовая реализация
var sf singleflight.Group
data, err, _ = sf.Do(fileID, func() (any, error) {
    return fetchFromOrigin(fileID)
})
```

### 6.4 Cache Key

```
{file_id}:{version}:{transform_params}
```

Пример: `abc123:v3:thumb_256x256`

### 6.5 TTL политика

| Тип файла | TTL |
|-----------|-----|
| Статика (js, css) | 1 год |
| Изображения | 30 дней |
| Видео | 7 дней |
| Приватные файлы | не кэшируются |

### 6.6 Cache Headers

```
Cache-Control: public, max-age=2592000, immutable
ETag: "{file_hash}"
Last-Modified: {upload_time}
Vary: Accept-Encoding
```

### 6.7 Инвалидация

```
Origin → Controller → все Edge ноды (параллельно)
POST /api/v1/invalidate { "file_id": "abc123" }

Edge нода: удалить из L1 и L2, следующий запрос → miss → origin
```

---

## 7. Безопасность

### 7.1 Контроль доступа к файлам

**Публичные файлы:** прямой доступ по URL
```
GET /files/{file_id}
```

**Приватные файлы:** подписанные URL (Signed URL)
```
GET /files/{file_id}?token={hmac_sha256}&expires={unix_ts}

token = HMAC-SHA256(file_id + expires + user_id, secret_key)
```

Параметры signed URL:
- `expires` — unix timestamp истечения
- `ip` — опционально привязать к IP
- `once` — одноразовый токен (хранится в Redis)

### 7.2 Защита от DDoS / брутфорса

**Rate Limiting (Redis + Sliding Window):**

```yaml
rate_limiting:
  download:
    per_ip:   100 req/min
    per_user: 500 req/min
  upload:
    per_ip:   10 req/min
    per_user: 50 req/min
```

**Connection Limiting:**
- Max connections per IP: 50
- Slow connection detection: минимум 1KB/s, иначе дроп через 30s
- FastHTTP `ReadTimeout`, `WriteTimeout`, `IdleTimeout`

### 7.3 Защита от Hotlinking

```go
// Whitelist доменов в конфиге
allowedReferers := config.GetAs("hotlink.allowed_domains", []string{})

referer := string(ctx.Request.Header.Peek("Referer"))
if !isAllowedReferer(referer, allowedReferers) {
    ctx.SetStatusCode(403)
    return
}
```

### 7.4 Защита от Path Traversal / Injection

- Валидация `file_id` regex: `^[a-zA-Z0-9_-]{8,64}$`
- Никакой прямой работы с путями ФС через пользовательский ввод
- Все пути через MinIO SDK

### 7.5 Content Security

- `X-Content-Type-Options: nosniff`
- `Content-Disposition: attachment` для исполняемых файлов
- MIME-type whitelist — блокировка загрузки HTML, SVG с JS, скриптов
- Scan на embedded malware в изображениях (ImageMagick policy)

### 7.6 TLS

- TLS 1.2+ обязательно, 1.3 предпочтительно
- HSTS: `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- Автоматическое обновление сертификатов (Let's Encrypt / ACME)
- Cert pinning опционально для мобильных клиентов

### 7.7 Аутентификация API

- Управляющие эндпоинты (upload, delete) — через sai-auth (JWT/Reference tokens)
- Межсервисное взаимодействие — service tokens с ротацией
- Admin API — IP whitelist + токен

### 7.8 Защита от Zip Bomb / Decompression Bomb

```go
const maxDecompressedSize = 1 << 30 // 1GB
reader := io.LimitReader(r, maxDecompressedSize)
```

### 7.9 Bot Protection

Разграничение легитимных ботов и вредоносных скраперов/краулеров.

- Whitelist легитимных User-Agent (Googlebot, Bingbot) с верификацией через rDNS
- Блокировка по паттернам: слишком высокий RPS с одного IP, нет заголовков браузера
- Honeypot URL — скрытая ссылка в HTML, реальный пользователь никогда не перейдёт
- Блокировка по репутационным спискам IP (хранятся в Redis, обновляются из публичных фидов)
- Режимы: `log` (только фиксировать) и `block` (возвращать 403/429)

```go
type BotRule struct {
    UAPattern   string `bson:"ua_pattern"`   // regex на User-Agent
    Action      string `bson:"action"`       // allow|block|log
    VerifyRDNS  bool   `bson:"verify_rdns"`  // проверить обратный DNS
}
```

---

## 8. Оптимизации производительности

### 8.1 Отдача файлов

- `sendfile()` syscall через FastHTTP (zero-copy)
- Streaming для больших файлов (chunked transfer)
- Range requests: `Accept-Ranges: bytes`, поддержка `Range: bytes=X-Y`
- Gzip/Brotli сжатие для текстовых типов (inline, не файлов)

### 8.2 Трансформации изображений (на лету)

Трансформации применяются по slug пресета или явным параметрам:

```
GET /files/{id}/p/{preset_slug}          # по пресету
GET /files/{id}/thumb/256x256            # явный resize
GET /files/{id}/thumb/256x256/webp       # resize + конвертация формата
GET /files/{id}/thumb/0x256             # пропорциональное масштабирование
```

- libvips (через govips) — самый быстрый вариант
- Результат кэшируется в L1/L2 с ключом `{id}:preset:{slug}` или `{id}:thumb:{params}`
- Ограничение: максимум 4096x4096 для защиты от DoS

### 8.3 Пресеты изображений

Пресеты создаются в админке и хранятся в MongoDB + Redis (hot cache).

```go
type ImagePreset struct {
    ID          primitive.ObjectID `bson:"_id"`
    Slug        string             `bson:"slug"`         // "avatar-sm", "product-card"
    Name        string             `bson:"name"`
    Width       int                `bson:"width"`        // 0 = пропорционально
    Height      int                `bson:"height"`       // 0 = пропорционально
    Format      string             `bson:"format"`       // jpeg|webp|png|avif|source
    Quality     int                `bson:"quality"`      // 1-100
    Fit         string             `bson:"fit"`          // cover|contain|fill|crop
    CropAnchor  string             `bson:"crop_anchor"`  // center|top|bottom|left|right
    Watermark   *WatermarkConfig   `bson:"watermark,omitempty"`
    StripMeta   bool               `bson:"strip_meta"`   // убрать EXIF
    Active      bool               `bson:"active"`
    CrTime      int64              `bson:"cr_time"`
}

type WatermarkConfig struct {
    FileID   string  `bson:"file_id"`   // ID файла водяного знака
    Position string  `bson:"position"`  // top-left|top-right|bottom-left|bottom-right|center
    Opacity  float64 `bson:"opacity"`   // 0.0 - 1.0
    Scale    float64 `bson:"scale"`     // 0.0 - 1.0, относительно ширины
}
```

**Admin API пресетов изображений:**
```
POST   /api/v1/admin/presets/image          # создать пресет
GET    /api/v1/admin/presets/image          # список пресетов
PUT    /api/v1/admin/presets/image/:slug    # обновить
DELETE /api/v1/admin/presets/image/:slug    # удалить (инвалидирует кэш)
```

### 8.4 Пресеты видео

Видеотранскодирование — async через очередь (Action System). FFmpeg как бэкенд.

```go
type VideoPreset struct {
    ID         primitive.ObjectID `bson:"_id"`
    Slug       string             `bson:"slug"`        // "hd-720", "mobile-360"
    Name       string             `bson:"name"`
    Width      int                `bson:"width"`
    Height     int                `bson:"height"`
    Codec      string             `bson:"codec"`       // h264|h265|vp9|av1
    Bitrate    string             `bson:"bitrate"`     // "2000k", "0" (CRF режим)
    CRF        int                `bson:"crf"`         // 0-51, качество в CRF режиме
    FPS        int                `bson:"fps"`         // 0 = исходный
    AudioCodec string             `bson:"audio_codec"` // aac|opus|mp3
    AudioBitrate string           `bson:"audio_bitrate"` // "128k"
    Container  string             `bson:"container"`   // mp4|webm|hls
    HLS        *HLSConfig         `bson:"hls,omitempty"`
    Active     bool               `bson:"active"`
    CrTime     int64              `bson:"cr_time"`
}

type HLSConfig struct {
    SegmentDuration int `bson:"segment_duration"` // секунды, default 6
    Playlist        string `bson:"playlist"`      // vod|event|live
}
```

**Готовые пресеты по умолчанию:**

| Slug | Разрешение | Codec | Bitrate | Назначение |
|------|-----------|-------|---------|-----------|
| `source` | оригинал | — | — | без транскодирования |
| `hd-1080` | 1920x1080 | h264 | 4000k | десктоп HD |
| `hd-720` | 1280x720 | h264 | 2000k | универсальный |
| `sd-480` | 854x480 | h264 | 800k | мобильный |
| `mobile-360` | 640x360 | h264 | 400k | слабый интернет |
| `hls-adaptive` | все выше | h264 | adaptive | HLS с переключением качества |

**Запрос транскодирования:**
```
POST /api/v1/files/{id}/transcode
{ "preset": "hd-720" }
→ { "job_id": "...", "status": "pending" }

GET /api/v1/files/{id}/transcode/{job_id}
→ { "status": "processing", "progress": 42 }

GET /files/{id}/p/hd-720       # отдача транскодированного файла
GET /files/{id}/p/hls-adaptive/playlist.m3u8  # HLS плейлист
```

**Admin API пресетов видео:**
```
POST   /api/v1/admin/presets/video          # создать пресет
GET    /api/v1/admin/presets/video          # список пресетов
PUT    /api/v1/admin/presets/video/:slug    # обновить
DELETE /api/v1/admin/presets/video/:slug    # удалить
```

**Очередь транскодирования:**
- Задачи через sai-queue-manager (уже в стеке)
- Параллельность: N воркеров (настраивается)
- Приоритеты: платные пользователи выше
- При провале — retry 3 раза с backoff

### 8.5 Авто WebP/AVIF

Конвертация изображений без явного указания формата — по заголовку `Accept` браузера.

```
Accept: image/avif,image/webp,*/*
→ автоматически отдаётся AVIF (если поддерживается)
→ иначе WebP
→ иначе оригинальный формат
```

- Конвертированный вариант кэшируется с ключом `{id}:auto:{format}`
- Выигрыш: 40–60% меньше трафика без изменения кода клиента
- `Vary: Accept` в ответе — браузеры и прокси кэшируют корректно

### 8.6 JS/CSS минификация

Для CDN, раздающего веб-ассеты (JS, CSS, HTML).

- Минификация при первом запросе, результат кэшируется
- JS: [tdewolff/minify](https://github.com/tdewolff/minify) — Go нативный, без NodeJS
- CSS: тот же пакет
- Включается через конфиг или заголовок `X-Minify: true`
- Только для текстовых MIME-типов (`text/css`, `application/javascript`)

### 8.7 Smart Preloader

После загрузки на origin — автоматически push на ближайшие edge ноды по популярности:
- Новый файл → push на 2-3 ноды ближайшего региона
- Файл с высоким hit rate (топ 10% по запросам) → push на все ноды
- Статистика популярности обновляется в Redis каждые 5 минут

### 8.8 Дедупликация

- SHA-256 хэш файла при загрузке
- Если хэш уже существует → сохранить только метаданные, файл не дублировать
- Экономия: до 40% места для пользовательского контента

---

## 9. Метаданные файлов (MongoDB)

```go
type FileRecord struct {
    ID          primitive.ObjectID `bson:"_id"`
    InternalID  string             `bson:"internal_id"`
    OwnerID     string             `bson:"owner_id"`
    Name        string             `bson:"name"`
    OriginalName string            `bson:"original_name"`
    MimeType    string             `bson:"mime_type"`
    Size        int64              `bson:"size"`
    Hash        string             `bson:"hash"` // SHA-256, для дедупликации
    StoragePath string             `bson:"storage_path"`
    Bucket      string             `bson:"bucket"`
    Public      bool               `bson:"public"`
    Tags        []string           `bson:"tags"`
    Meta        map[string]any     `bson:"meta"` // EXIF, duration, etc.
    Status      string             `bson:"status"` // pending|ready|deleted
    CrTime      int64              `bson:"cr_time"`
    ChTime      int64              `bson:"ch_time"`
    DeletedAt   *int64             `bson:"deleted_at,omitempty"`
}
```

---

## 10. Конфигурация

### config.template.yml (origin)

```yaml
name: "${SERVICE_NAME}"
version: "${SERVICE_VERSION}"

server:
  http:
    host: "${SERVER_HOST}"
    port: ${SERVER_PORT}

storage:
  type: "${STORAGE_TYPE}"          # minio | s3 | gcs
  endpoint: "${STORAGE_ENDPOINT}"
  access_key: "${STORAGE_ACCESS_KEY}"
  secret_key: "${STORAGE_SECRET_KEY}"
  bucket: "${STORAGE_BUCKET}"
  region: "${STORAGE_REGION}"

upload:
  max_file_size: ${MAX_FILE_SIZE}  # bytes, default 5368709120 (5GB)
  chunk_size: ${CHUNK_SIZE}        # bytes, default 10485760 (10MB)
  allowed_mime_types: "${ALLOWED_MIME_TYPES}"
  antivirus_enabled: ${ANTIVIRUS_ENABLED}
  deduplication: ${DEDUPLICATION_ENABLED}

signed_url:
  secret: "${SIGNED_URL_SECRET}"
  default_ttl: ${SIGNED_URL_TTL}   # seconds, default 3600

cdn:
  controller_url: "${CDN_CONTROLLER_URL}"
  region: "${CDN_REGION}"

rate_limiting:
  download_per_ip: ${RATE_LIMIT_DOWNLOAD_IP}
  upload_per_ip: ${RATE_LIMIT_UPLOAD_IP}

hotlink:
  enabled: ${HOTLINK_PROTECTION_ENABLED}
  allowed_domains: "${HOTLINK_ALLOWED_DOMAINS}"

cache:
  enabled: ${CACHE_ENABLED}
  type: "redis"

database:
  type: "mongodb"
  connection_string: "${MONGODB_CONNECTION_STRING}"
```

---

## 11. Репликация и консистентность

### Стратегия

- **Origin → Edge:** eventual consistency, async push через Action System
- **Гарантия доступности:** файл всегда доступен на origin, edge — кэш
- **Инвалидация:** синхронная (controller ждёт подтверждения от всех нод) с таймаутом 5s

### Failover

```
Edge cache miss
  → запрос к origin
  → origin недоступен?
    → запрос к соседнему edge региону
    → если и там нет → 404/503
```

---

## 12. Мониторинг и метрики

### Ключевые метрики (Prometheus)

```
cdn_requests_total{node, status, method}
cdn_cache_hit_ratio{node}
cdn_upload_bytes_total{node}
cdn_download_bytes_total{node}
cdn_file_size_bytes{bucket} (histogram)
cdn_upload_duration_seconds (histogram)
cdn_node_latency_seconds{node}
cdn_active_connections{node}
```

### Алерты

| Условие | Severity |
|---------|----------|
| cache_hit_ratio < 0.7 | warning |
| node latency > 200ms | warning |
| node latency > 500ms | critical |
| upload errors > 1% | warning |
| disk_free < 10GB | critical |

### Реалтайм аналитика (Admin Dashboard)

Эндпоинты для дашборда, данные из Redis (realtime) + MongoDB (история):

```
GET /api/v1/admin/analytics/realtime
→ {
    requests_per_second: 1240,
    bandwidth_mbps: 340,
    cache_hit_ratio: 0.94,
    active_connections: 8420,
    top_files: [...],        # топ 10 файлов по запросам
    requests_by_country: {}, # география
    requests_by_node: {}     # нагрузка по нодам
  }

GET /api/v1/admin/analytics/history?from=...&to=...&granularity=1h
→ временной ряд метрик

GET /api/v1/admin/analytics/files/:id
→ статистика по конкретному файлу (просмотры, bandwidth, топ стран)

GET /api/v1/admin/analytics/bandwidth?group_by=region
→ трафик по регионам (для биллинга)
```

---

## 13. Схема развёртывания

```yaml
# docker-compose (dev/staging)
services:
  sai-cdn-origin:
    build: ./sai-cdn-origin
    ports: ["8085:8080"]
    depends_on: [mongodb, redis, minio]

  sai-cdn-edge:
    build: ./sai-cdn-edge
    ports: ["8086:8080"]
    depends_on: [redis, sai-cdn-origin]

  sai-cdn-controller:
    build: ./sai-cdn-controller
    ports: ["8087:8080"]
    depends_on: [consul, redis, mongodb]

  minio:
    image: minio/minio
    ports: ["9000:9000", "9001:9001"]

  consul:
    image: consul:1.17
    ports: ["8500:8500"]
```

**Production:** Docker Swarm + Traefik. Каждый сервис — отдельный `docker service`, масштабирование через `docker service scale sai-cdn-edge=N`. Traefik автоматически подхватывает новые реплики через Swarm labels.

---

## 14. Этапы разработки

| Этап | Задачи | Срок |
|------|--------|------|
| **MVP** | sai-cdn-origin (upload/download), MinIO, signed URLs | 2 нед |
| **Edge** | sai-cdn-edge: L1/L2 кэш, проксирование, Request Coalescing | 1 нед |
| **Shield** | Origin Shielding нода, Perma-Cache | 1 нед |
| **Security** | Rate limiting, hotlink protection, DDoS защита | 1 нед |
| **Controller** | sai-cdn-controller, выбор ноды, health checks | 1 нед |
| **Transforms** | Пресеты изображений, авто WebP/AVIF, JS/CSS минификация | 1 нед |
| **Video** | Пресеты видео, FFmpeg очередь, HLS | 1.5 нед |
| **Analytics** | Реалтайм дашборд, история, статистика по файлам | 0.5 нед |
| **Observability** | Метрики Prometheus, алерты, Grafana дашборды | 0.5 нед |
| **Load testing** | k6 / wrk, Smart Preloader тюнинг, оптимизация | 0.5 нед |
