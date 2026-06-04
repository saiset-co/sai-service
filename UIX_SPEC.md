# SAI UIX — Техническое задание

## Цель

Серверная компонентная система для сборки HTML-страниц из переиспользуемых компонентов.
Go знает всё дерево компонентов на момент рендеринга → вставляет только нужные CSS/JS → отдаёт готовый HTML без клиентского монтирования.

---

## Принципы

- Каждый компонент — Go-структура с методами `Render()`, `CSS()`, `JS()`
- Компоненты вкладываются друг в друга (дерево)
- Страница рекурсивно обходит дерево, собирает уникальный список ассетов
- CSS вставляется в `<head>`, JS — перед `</body>`
- Ни `core.js`, ни `manifest.js`, ни `data-sai-widget` не нужны
- JS-файлы компонентов — только для интерактивности (обработчики событий), не для рендеринга

---

## Интерфейс компонента

```go
// Component — базовый интерфейс любого элемента
type Component interface {
    Render() template.HTML
    Assets() ComponentAssets
}

// ComponentAssets — ассеты самого компонента (без дочерних)
type ComponentAssets struct {
    CSS []string // пути к .css файлам относительно /assets/uix/
    JS  []string // пути к .js файлам относительно /assets/uix/
}
```

---

## Сборка страницы

```go
type Page struct {
    BasePath string
    Head     template.HTML // дополнительный HTML в <head>
    Root     Component
}

// Build собирает готовый HTML: обходит дерево компонентов,
// дедуплицирует ассеты, вставляет <link>/<script>, рендерит Root
func (p *Page) Build() template.HTML
```

Порядок обхода ассетов: сначала родитель, потом дочерние (CSS каскадируется правильно).

---

## Дерево компонентов

Полная иерархия от страницы до атомарных примитивов.

```
Page
└── Layout
    ├── Header
    │   ├── Logo          → Image + Link
    │   ├── Nav (horizontal)
    │   │   └── NavItem   → Icon? + Link
    │   └── UserMenu      → Avatar + Text + Link[]
    ├── Sidebar
    │   ├── Nav (vertical)
    │   │   ├── NavGroup  → Label + NavItem[]
    │   │   └── NavItem   → Icon? + Link
    │   └── SidebarFooter → Text + Link[]
    ├── Main
    │   ├── Breadcrumbs   → Crumb[] → Link | Text
    │   ├── PageHeader    → H1 + P? + Button[]
    │   ├── Alert         → Icon + Text + Button?
    │   ├── Section
    │   │   ├── SectionHeader → H2 + Button[]
    │   │   └── Content   → Widget[]
    │   └── Tabs
    │       └── Tab       → Link | Button
    └── Footer
        ├── Text
        └── Link[]
```

### Структурные компоненты
```
Container           max-width обёртка
Grid                → GridItem[]
Flex                → FlexItem[]
Stack               вертикальный стек компонентов
Split               два столбца
Divider             горизонтальный разделитель
Spacer              пустое пространство
```

### Компоненты наложения
```
Modal
├── ModalHeader     → H2 + CloseButton
├── ModalBody       → Component[]
└── ModalFooter     → Button[]

Drawer              как Modal, но со стороны экрана
Toast               → Icon + Text + CloseButton
Tooltip             → Text (по hover)
ConfirmDialog       → Modal с двумя кнопками
```

### Примитивы (атомарные, без дочерних UIX-компонентов)
```
── Текст ──────────────────────────────────
H1 H2 H3 H4 H5 H6   заголовки
P                    параграф
Text                 строка (span)
Strong               жирный
Em                   курсив
Code                 inline code
Pre                  блок кода
Blockquote           цитата
Time                 дата/время с datetime атрибутом

── Интерактив ─────────────────────────────
Link                 <a href> с обязательным href
Button               variant: primary|secondary|danger|warning|ghost
Icon                 символ или SVG (title обязателен для a11y)
CloseButton          специализированная кнопка ×

── Медиа ──────────────────────────────────
Image                alt обязателен, loading=lazy по умолчанию
Video                controls, poster
Avatar               Image с fallback на инициалы

── Индикаторы ─────────────────────────────
Badge                ok|warn|danger|info|default
Dot                  цветная точка статуса
ProgressBar          percent + tone (auto)
Spinner              индикатор загрузки

── Форм-примитивы ─────────────────────────
Input                type: text|email|password|number|tel|url
Textarea             многострочный ввод
Select               список опций
Checkbox             флаг
Radio                переключатель
Switch               toggle on/off
FileInput            загрузка файла
HiddenInput          скрытое поле
DateInput            дата
SearchInput          поиск с кнопкой очистки
Label                подпись поля
FieldError           текст ошибки поля
FieldHint            подсказка под полем
```

---

## Виджеты

Виджет — готовый набор примитивов и компонентов для быстрой сборки типовых блоков.
Не требует верстки вручную — только данные.

### Контентные виджеты

```
Hero
├── H1              главный заголовок
├── P               подзаголовок
├── Button[]        CTA кнопки
└── Image?          фоновое или декоративное изображение

ScrollGallery
└── []ImageWithCaption
    ├── Image       alt обязателен
    └── Text        подпись

TextScrollGallery
├── H2?
├── P?              текст слева
└── []ImageWithCaption

Gallery             сетка изображений
└── []ImageWithCaption

Carousel
├── []ImageWithCaption
├── PrevButton
└── NextButton

Timeline
└── []TimelineItem
    ├── Dot         цвет по статусу
    ├── Time
    ├── Text        заголовок события
    └── P?          описание

EmptyState          пустое состояние списка
├── Icon?
├── H2
├── P
└── Button?

CodeBlock
├── Pre             код
├── Badge           язык
└── CopyButton
```

### Виджеты данных

```
Table
├── Caption | H2?
├── Toolbar
│   ├── SearchInput?
│   └── Button[]    действия заголовка
├── TableHead
│   └── Th[]
├── TableBody
│   └── []Tr
│       ├── Td[]    ячейки (Text|Badge|Link|Button|Image|Code)
│       └── ActionsTd
│           └── Button[]
└── Pagination
    ├── Text        "1–20 из 150"
    └── Button[]    страницы

StatGrid            сетка KPI-карточек
└── []StatCard
    ├── Text        значение
    ├── Text        подпись
    └── Badge?      тон (ok/warn/danger)

MetricCard          одна метрика с прогрессом
├── Text            подпись
├── ProgressBar
├── Text            значение
└── Text?           пояснение

KeyValueList        список ключ-значение
└── []KeyValue
    ├── Label
    └── Text | Badge | Code | Link

LogViewer           тёмный терминал с прокруткой
├── StatusBar       → Badge(статус) + Text
├── Pre             содержимое лога
└── CloseButton

DataExport          кнопка скачивания
└── Button          CSV | JSON | XLSX
```

### Навигационные виджеты

```
Breadcrumbs         + JSON-LD BreadcrumbList автоматически
└── []Crumb
    ├── Link        (все кроме последнего)
    └── Text        (текущая страница)

Tabs
└── []Tab
    ├── Link | Button
    └── Badge?      счётчик

Pagination          отдельный виджет (вне Table)
├── Text            диапазон
└── []Button
```

### Форм-виджеты

```
Form
├── FormSection[]
│   ├── H3?
│   └── Field[]
│       ├── Label
│       ├── Input | Select | Textarea | Switch | ...
│       ├── FieldHint?
│       └── FieldError?
└── FormActions
    └── Button[]    Submit + Cancel

DropdownSection     сворачиваемая секция фильтров
├── Caption | H3
└── []Field

FilterPanel
└── []DropdownSection

SearchForm
├── SearchInput
└── Button

LoginForm
├── Field(email)
├── Field(password)
├── Link            "Забыли пароль?"
└── Button(primary) "Войти"

RegisterForm
├── Field(name)
├── Field(email)
├── Field(password)
├── Field(password_confirm)
└── Button(primary)

InlineEdit          клик по тексту → поле ввода
├── Text            режим просмотра
├── Input           режим редактирования
├── Button(save)
└── Button(cancel)

TagInput            ввод с тегами
├── []Tag           выбранные значения
└── SearchInput     поиск/добавление

FileUpload
├── FileInput       drag & drop зона
├── ProgressBar?
└── []FileItem      загруженные файлы → Text + RemoveButton

RichSelect          Select с поиском
├── SearchInput
├── []SelectOption  Text + Checkbox
└── Button          применить

DateRangePicker
├── DateInput       от
└── DateInput       до

BulkActions         действия над выбранными строками таблицы
├── Checkbox        выбрать все
├── Text            "Выбрано: N"
└── Button[]        действия
```

### Пример сборки страницы

```go
page := uix.NewPage("/catalog", uix.Layout(
    uix.Sidebar(
        uix.Nav(
            uix.NavGroup("Каталог",
                uix.NavItem("Товары", "/catalog/products", uix.Icon("📦")),
                uix.NavItem("Категории", "/catalog/categories", uix.Icon("📂")),
            ),
        ),
    ),
    uix.Main(
        uix.Breadcrumbs(
            uix.Crumb("Главная", "/"),
            uix.Crumb("Каталог", "/catalog"),
            uix.Crumb("Товары", ""),
        ),
        uix.PageHeader("Товары",
            uix.Button("Добавить", "primary").WithOnClick("openModal('create')"),
        ),
        widgets.Table(cols, rows).
            WithSearch().
            WithID("products-table").
            WithSource(func(ctx) []Row { return productRows(repo.List(ctx)) }),
    ),
)).WithSEO(uix.SEO{
    Title:       "Товары | Каталог",
    Description: "Управление товарами каталога",
})
```

---

## Структура файлов пакета

```
sai-service/uix/
├── component.go        # Component interface, ComponentAssets, Page
├── base.go             # BaseComponent — вспомогательная embed-структура
└── components/
    ├── layout/
    │   ├── Layout.go
    │   ├── Layout.css
    │   └── Layout.js       # опционально, если нужен JS
    ├── nav/
    │   ├── Nav.go
    │   ├── Nav.css
    │   └── Nav.js
    ├── table/
    │   ├── Table.go
    │   ├── Table.css
    │   └── Table.js         # поиск/пагинация по готовому DOM
    ├── page_header/
    │   ├── PageHeader.go
    │   └── PageHeader.css
    ├── stat_grid/
    │   ├── StatGrid.go
    │   └── StatGrid.css
    ├── card/
    │   ├── Card.go
    │   └── Card.css
    ├── metric_card/
    │   ├── MetricCard.go
    │   └── MetricCard.css
    └── log_viewer/
        ├── LogViewer.go
        ├── LogViewer.css
        └── LogViewer.js     # polling, show/hide
```

CSS и JS файлы встроены через `//go:embed` в соответствующем `.go` файле компонента.

---

## BaseComponent

```go
// BaseComponent упрощает реализацию простых компонентов без дочерних
type BaseComponent struct {
    css []string
    js  []string
}

func (b *BaseComponent) Assets() ComponentAssets {
    return ComponentAssets{CSS: b.css, JS: b.js}
}
```

---

## Пример реализации компонента

```go
// components/table/Table.go
package table

import (
    _ "embed"
    "html/template"
    "strings"
    "html"

    "github.com/saiset-co/sai-service/uix"
)

//go:embed Table.css
var tableCss string

//go:embed Table.js
var tableJs string

type Column struct {
    Title string
}

type Row struct {
    Cells   []any
    Actions []Action
}

type Action struct {
    Label   string
    Href    string
    OnClick string
    Variant string
}

type Table struct {
    columns      []Column
    rows         []Row
    emptyMessage string
    searchable   bool
}

func New(columns []Column, rows []Row) *Table {
    return &Table{columns: columns, rows: rows, emptyMessage: "Нет записей."}
}

func (t *Table) WithSearch() *Table {
    t.searchable = true
    return t
}

func (t *Table) Assets() uix.ComponentAssets {
    assets := uix.ComponentAssets{CSS: []string{"table/Table.css"}}
    if t.searchable {
        assets.JS = []string{"table/Table.js"}
    }
    return assets
}

func (t *Table) Render() template.HTML {
    var b strings.Builder
    // ... рендеринг <table> ...
    return template.HTML(b.String())
}
```

---

## Пример реализации Layout с дочерними компонентами

```go
// components/layout/Layout.go
package layout

import (
    _ "embed"
    "html/template"
    "strings"

    "github.com/saiset-co/sai-service/uix"
)

//go:embed Layout.css
var layoutCss string

type Layout struct {
    children []uix.Component
}

func New(children ...uix.Component) *Layout {
    return &Layout{children: children}
}

func (l *Layout) Assets() uix.ComponentAssets {
    return uix.ComponentAssets{CSS: []string{"layout/Layout.css"}}
}

func (l *Layout) Render() template.HTML {
    var b strings.Builder
    b.WriteString(`<div class="sai-layout">`)
    for _, child := range l.children {
        b.WriteString(string(child.Render()))
    }
    b.WriteString(`</div>`)
    return template.HTML(b.String())
}

// Children возвращает дочерние компоненты — Page использует это для сбора ассетов
func (l *Layout) Children() []uix.Component {
    return l.children
}
```

---

## Page.Build() — алгоритм

```go
func (p *Page) Build() template.HTML {
    // 1. Рекурсивный обход дерева, сбор уникальных ассетов
    css, js := collectAssets(p.Root, map[string]bool{})

    // 2. Рендеринг компонента
    body := p.Root.Render()

    // 3. Сборка страницы
    var out strings.Builder
    out.WriteString(`<!doctype html><html><head>`)
    out.WriteString(string(p.Head))
    for _, path := range css {
        out.WriteString(`<link rel="stylesheet" href="` + p.BasePath + `/assets/uix/` + path + `">`)
    }
    out.WriteString(`</head><body>`)
    out.WriteString(string(body))
    for _, path := range js {
        out.WriteString(`<script src="` + p.BasePath + `/assets/uix/` + path + `"></script>`)
    }
    out.WriteString(`</body></html>`)
    return template.HTML(out.String())
}
```

Интерфейс для компонентов с дочерними:

```go
// ParentComponent — компонент, имеющий дочерние
type ParentComponent interface {
    Component
    Children() []Component
}
```

`collectAssets` проверяет через type assertion наличие `Children()` и рекурсирует.

---

## Раздача ассетов

Каждый компонент встраивает свои CSS/JS через `//go:embed`.
`uix.RegisterAssets(mux)` регистрирует один HTTP-хендлер `/assets/uix/*` который отдаёт файлы из embed.FS всех зарегистрированных компонентов.

```go
// В main.go сервиса
uix.RegisterAssets(sai.Router())

// Либо автоматически при первом использовании компонента
```

---

## SEO

### Метаданные страницы

Каждая страница имеет SEO-конфиг, который `Build()` вставляет в `<head>`:

```go
type SEO struct {
    Title       string // <title> и og:title
    Description string // <meta name="description"> и og:description
    Canonical   string // <link rel="canonical">
    Lang        string // <html lang="..."> (default: "ru")
    NoIndex     bool   // <meta name="robots" content="noindex,nofollow">

    OGImage string // og:image
    OGType  string // og:type (default: "website")

    JSONLD []JSONLDSchema // <script type="application/ld+json">
}

type JSONLDSchema struct {
    Type    string
    Payload map[string]any
}

page := uix.NewPage("/", layout).WithSEO(uix.SEO{
    Title:       "Деплой | My Service",
    Description: "Управление задачами деплоя",
    Canonical:   "https://example.com/admin/pages/deploy",
    OGImage:     "https://example.com/og.png",
})
```

Генерируемые теги:
```html
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Деплой | My Service</title>
  <meta name="description" content="Управление задачами деплоя">
  <link rel="canonical" href="https://example.com/admin/pages/deploy">
  <meta property="og:title" content="Деплой | My Service">
  <meta property="og:description" content="Управление задачами деплоя">
  <meta property="og:image" content="https://example.com/og.png">
  <meta property="og:type" content="website">
  <meta name="twitter:card" content="summary_large_image">
  <script type="application/ld+json">{"@context":"https://schema.org",...}</script>
</head>
```

`<meta charset>` и `<meta viewport>` всегда вставляются автоматически — нельзя забыть.

---

### Валидация дерева заголовков

`Build()` в режиме `ENVIRONMENT=dev` парсит отрендеренный HTML и проверяет иерархию H1–H6.

**Правила:**
- Ровно один `<h1>` на странице
- Нет пропусков уровней (H1 → H3 без H2 — ошибка)
- H1 не пустой

**Поведение при нарушении:**
- `dev`: `sai.Logger().Warn(...)` + HTML-комментарий `<!-- SEO WARNING: ... -->`
- `prod`: только лог, HTML не меняется

```go
type HeadingIssue struct {
    Level   int
    Text    string
    Problem string // "duplicate_h1" | "level_skip" | "empty_h1"
}

func (p *Page) BuildWithDiagnostics() (template.HTML, []HeadingIssue)
```

---

### Sitemap

Каждая страница при регистрации может заявить себя в sitemap:

```go
router.GET("/about", handler).
    WithSEO(uix.SEO{Title: "О нас"}).
    WithSitemap(uix.SitemapEntry{
        ChangeFreq: "monthly",
        Priority:   0.8,
    })

// Автоматический эндпоинт /sitemap.xml
uix.RegisterSitemap(sai.Router(), "https://example.com")
```

Динамические страницы (каталог, блог):
```go
uix.AddDynamicSitemap(func() []uix.SitemapEntry {
    posts, _ := postRepo.ListAll()
    // возвращает записи для каждого поста
})
```

Генерирует стандартный `sitemap.xml` по запросу.

---

### robots.txt

```go
uix.RegisterRobots(sai.Router(), uix.RobotsConfig{
    Rules: []uix.RobotsRule{
        {UserAgent: "*", Allow: []string{"/"}, Disallow: []string{"/admin/", "/api/"}},
    },
    Sitemap: "https://example.com/sitemap.xml",
})
```

---

### Хлебные крошки

Компонент генерирует HTML + schema.org `BreadcrumbList` JSON-LD автоматически:

```go
bc := uix.Breadcrumbs(
    uix.Crumb("Главная", "/"),
    uix.Crumb("Деплой", "/deploy"),
    uix.Crumb("Задача #42", ""), // текущая страница — без href
)
// <nav aria-label="breadcrumb"> + JSON-LD BreadcrumbList в <head>
```

---

### Прочие SEO-механизмы

| Механизм | Реализация |
|---|---|
| Preload критических ассетов | `Page.WithPreload("font.woff2", "font")` → `<link rel="preload">` |
| `loading="lazy"` для изображений | компонент `uix.Image` добавляет автоматически |
| Alt-текст | `uix.Image(src, alt)` — alt обязательный параметр на уровне Go API |
| `noindex` через HTTP заголовок | `X-Robots-Tag: noindex` при `SEO.NoIndex = true` |
| Пагинация | `Page.WithPagination(prev, next)` → `rel="prev"` / `rel="next"` |
| `hreflang` | `SEO.Alternates []Alternate` → `<link rel="alternate" hreflang="...">` |

---

## Partial Render (фрагментный рендеринг)

Механизм частичного обновления страницы без перезагрузки: форма отправляется через AJAX,
сервер перерисовывает только нужный компонент и возвращает HTML-фрагмент, JS заменяет элемент в DOM.

### Принцип работы

```
POST /tasks/create
  → сервер сохраняет в БД
  → вызывает taskTable.RenderFragment(ctx)
  → возвращает <div id="task-table">...</div>
  → JS заменяет старый элемент новым
```

Никакого WebSocket, SSE или клиентского store — обычный HTTP POST, ответ — готовый HTML.

### Go API

```go
// Компонент регистрируется с ID и функцией получения данных
taskTable := uix.Table(cols).
    WithID("task-table").
    WithSource(func(ctx *RequestCtx) []Row {
        return taskRows(taskRepo.List(ctx))
    })

// Полный рендер страницы (первая загрузка)
page := uix.NewPage("/tasks", layout).WithSEO(...)
ctx.HTML(page.Build())

// POST хендлер — после мутации возвращает только фрагмент
func (h *Handler) CreateTask(ctx *RequestCtx) {
    h.repo.Create(parseForm(ctx))
    ctx.HTML(taskTable.RenderFragment(ctx))
    // RenderFragment рендерит компонент вызывая WithSource,
    // оборачивает в <div id="task-table">...</div>
}
```

`RenderFragment` отличается от `Render` тем, что:
- вызывает `WithSource` для получения свежих данных
- оборачивает результат в корневой тег с `id`
- не рендерит `<html>`, `<head>`, `<body>`

### HTML форм

```html
<!-- data-refresh указывает какой элемент заменить после успешного POST -->
<form data-refresh="#task-table" action="/tasks/create" method="post">
  <input name="name" required>
  <button type="submit">Создать</button>
</form>

<div id="task-table">
  <!-- первичный рендер Go -->
</div>
```

### Клиентский JS (fragment.js, ~40 строк)

Универсальный хелпер, входит в базовые ассеты UIX. Подключается автоматически если на странице
есть хотя бы один компонент с `WithID`.

```js
(function () {
  document.addEventListener('submit', async function (e) {
    var form = e.target.closest('form[data-refresh]')
    if (!form) return
    e.preventDefault()

    var targetSel = form.getAttribute('data-refresh')
    var target = document.querySelector(targetSel)
    if (!target) return

    var btn = form.querySelector('[type=submit]')
    if (btn) btn.disabled = true

    try {
      var res = await fetch(form.action, {
        method: form.method || 'POST',
        headers: { 'X-Fragment': '1' },
        body: new FormData(form)
      })
      if (!res.ok) throw new Error(res.statusText)
      target.outerHTML = await res.text()
      form.reset()
    } catch (err) {
      // показать ошибку — компонент uix.FormError если есть
    } finally {
      if (btn) btn.disabled = false
    }
  })
})()
```

Заголовок `X-Fragment: 1` позволяет Go-хендлеру определить тип запроса:

```go
func IsFragmentRequest(ctx *RequestCtx) bool {
    return string(ctx.Request.Header.Peek("X-Fragment")) == "1"
}

// В хендлере можно обслуживать оба случая одним методом:
func (h *Handler) Tasks(ctx *RequestCtx) {
    if uix.IsFragmentRequest(ctx) {
        ctx.HTML(taskTable.RenderFragment(ctx))
        return
    }
    ctx.HTML(page.Build())
}
```

### Расширения

| Возможность | Реализация |
|---|---|
| Обновить несколько элементов | POST возвращает JSON `[{id, html}, ...]`, JS применяет все |
| Показать ошибку формы | сервер возвращает статус 422, JS вставляет HTML ошибки в `data-error` элемент |
| Оптимистичный UI | JS скрывает строку формы до ответа сервера (опционально) |
| Перезагрузка нескольких компонентов | `data-refresh="#table,#stats"` — JS свапает оба |

---

## Что НЕ входит в систему

- Клиентский рендеринг (никакого `data-sai-widget`, `SAI_UIX`, `core.js`)
- Гидрация (hydration) — состояние только на сервере или через явный JS
- Реактивность — обновление страницы через стандартный `fetch` + `location.reload()`
  или через явный HTMX/fetch в JS-файлах конкретных компонентов

---

## Кэширование и прогрев кэша

### Уровни кэша

Система поддерживает три независимых уровня кэширования. Каждый уровень можно включать отдельно.

```
Запрос
  → L1: HTTP Cache-Control (браузер / CDN)
  → L2: Page Cache (полный HTML страницы)
  → L3: Component Cache (HTML отдельного компонента)
  → рендеринг
```

---

### L3 — Кэш компонентов

Самый гранулярный уровень. Если компонент не зависит от пользователя — его HTML кэшируется:

```go
nav := uix.Nav(items).
    WithCache("nav-main", 5*time.Minute)

// RenderFragment и Render проверяют кэш перед вызовом WithSource
// Ключ кэша: "nav-main" (статичный) или с параметрами: "task-table:page=2"
taskTable := uix.Table(cols).
    WithID("task-table").
    WithSource(func(ctx *RequestCtx) []Row { ... }).
    WithCacheKey(func(ctx *RequestCtx) string {
        return "task-table:" + ctx.QueryArgs().String()
    }, 30*time.Second)
```

Кэш компонентов хранится в памяти (sync.Map) или Redis — настраивается глобально:

```go
uix.SetCacheBackend(uix.RedisCacheBackend(redisClient))
// или
uix.SetCacheBackend(uix.MemoryCacheBackend(maxItems: 1000))
```

---

### L2 — Кэш страниц

Кэширует полный HTML страницы. Подходит для публичных страниц без пользовательского контекста.

```go
page := uix.NewPage("/catalog", layout).
    WithSEO(...).
    WithPageCache(uix.PageCache{
        TTL:  10 * time.Minute,
        Tags: []string{"products", "categories"},
    })
```

При запросе `Build()` проверяет кэш по URL + Vary-заголовкам. При промахе рендерит и сохраняет.

---

### L1 — HTTP Cache-Control

Для полностью статичных страниц Go выставляет заголовки напрямую:

```go
page := uix.NewPage("/about", layout).
    WithHTTPCache(uix.HTTPCache{
        MaxAge:  24 * time.Hour,
        Public:  true,
        ETag:    true, // автогенерация ETag от хэша HTML
        VaryBy:  []string{"Accept-Language"},
    })
```

Генерирует:
```
Cache-Control: public, max-age=86400
ETag: "a3f8b2..."
Vary: Accept-Language
```

---

### Инвалидация по тегам

Ключевой механизм для точечной инвалидации без сброса всего кэша.

```go
// Компоненты и страницы объявляют теги
taskTable.WithCacheTags("tasks")
statsWidget.WithCacheTags("tasks", "servers")
page.WithPageCache(uix.PageCache{Tags: []string{"tasks"}})

// После мутации — инвалидировать всё связанное с тегом
func (h *Handler) CreateTask(ctx *RequestCtx) {
    h.repo.Create(parseForm(ctx))
    uix.InvalidateTag(ctx, "tasks")
    // taskTable и statsWidget и page — все сбросились
    ctx.HTML(taskTable.RenderFragment(ctx)) // перерендер свежих данных
}
```

Инвалидация по тегу атомарна — версионный счётчик в Redis, без перебора ключей:
```
cache key = "component:{name}:v{tag_version}:{params_hash}"
invalidate("tasks") → increment tasks_version
→ все старые ключи автоматически промахиваются
```

---

### Прогрев кэша (Cache Warming)

**Проблема:** после деплоя или инвалидации первый запрос к каждой странице — холодный.
На сложных страницах это может занять сотни миллисекунд.

**1. Pre-warming при старте сервиса**

```go
// В конфиге: список URL для прогрева
warmup:
  enabled: true
  concurrency: 4
  urls:
    - /
    - /catalog
    - /about
  dynamic:
    - source: "SELECT slug FROM products WHERE active=true"
      pattern: "/products/{slug}"

// Сервис при старте делает GET к себе через loopback до принятия внешнего трафика
uix.WarmupCache(ctx, warmupConfig)
```

**2. Background refresh (stale-while-revalidate)**

Самый эффективный паттерн для высоконагруженных страниц:

```go
page.WithPageCache(uix.PageCache{
    TTL:              5 * time.Minute,
    StaleWhileRevalidate: 30 * time.Second,
    // Отдаём устаревший кэш в течение 30с после истечения TTL,
    // в фоне запускаем перерендер.
    // Пользователь никогда не ждёт холодного рендера.
})
```

Реализация:
```
TTL истёк → запрос пришёл
  → отдаём stale HTML (быстро)
  → в горутине: рендерим → сохраняем в кэш
  → следующий запрос получит свежий HTML
```

**3. Event-driven прогрев**

После инвалидации тега — немедленно прогреть связанные страницы в фоне:

```go
uix.OnInvalidate("tasks", func() {
    uix.WarmupURLs([]string{"/tasks", "/dashboard"})
})
```

---

### Vary и пользовательский контекст

Страницы с персонализацией нельзя кэшировать на уровне L2. Варианты:

| Тип страницы | Стратегия |
|---|---|
| Публичная, одинакова для всех | L1 + L2 + L3 |
| Публичная, зависит от языка | L1 (Vary: Accept-Language) + L2 по ключу `lang` |
| Авторизованная, общие данные | L3 (компоненты без user-данных) |
| Авторизованная, персональная | Только L3 для статичных частей (nav, footer) |
| Страница с CSRF | Без L1/L2, только L3 |

Компонент может явно исключить себя из кэша:

```go
userMenu := uix.UserMenu(user).WithNoCache() // всегда рендерится свежим
```

---

### Метрики кэша

Кэш-слой эмитирует метрики в SAI Metrics автоматически:

```
uix_cache_hit_total{level="component|page", tag="tasks"}
uix_cache_miss_total{level="component|page", tag="tasks"}
uix_cache_render_duration_seconds{component="task-table"}
uix_cache_warmup_duration_seconds
```

---

## Что НЕ входит в систему

| # | Задача |
|---|--------|
| 1 | `uix/component.go` — интерфейсы `Component`, `ParentComponent`, `Page.Build()` |
| 2 | `uix/assets.go` — embed.FS регистрация и HTTP-хендлер `/assets/uix/*` |
| 3 | `uix/seo.go` — `SEO` struct, генерация тегов, валидация H1–H6 |
| 4 | `uix/sitemap.go` — `/sitemap.xml`, динамические источники |
| 5 | `uix/robots.go` — `/robots.txt` |
| 6 | `uix/fragment.go` — `WithID`, `WithSource`, `RenderFragment`, `IsFragmentRequest` |
| 7 | `uix/assets/fragment.js` — универсальный AJAX form submit (~40 строк) |
| 8 | `uix/cache.go` — L3 кэш компонентов, `WithCache`, `WithCacheTags`, `InvalidateTag` |
| 9 | `uix/page_cache.go` — L2 кэш страниц, `stale-while-revalidate`, прогрев |
| 10 | `uix/warmup.go` — прогрев при старте, event-driven прогрев, динамические URL |
| 11 | Базовые компоненты: `layout`, `nav`, `page_header`, `stat_grid` |
| 12 | `table` с поиском (JS на существующем DOM) |
| 13 | `card`, `metric_card`, `image` (alt обязателен) |
| 14 | `log_viewer` (polling JS), `modal` |
| 15 | `breadcrumbs` (HTML + JSON-LD BreadcrumbList) |
| 16 | Интеграция в `sai-service`: `Builder` переписывается на компонентах |
| 17 | Перенос `sai-control-uix` на новую систему |
