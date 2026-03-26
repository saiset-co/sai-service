package admin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/types"
)

type PageHandler func(ctx *types.RequestCtx) (*PageData, error)
type ResourceListHandler func(ctx *types.RequestCtx) ([]map[string]interface{}, error)

type Builder struct {
	group        types.GroupBuilder
	basePath     string
	title        string
	subtitle     string
	frameworkURL string
	headHTML     template.HTML
	authProvider string
	tmpl         *template.Template
	homePage     *PageConfig
	pages        []*PageConfig
	resources    []*ResourceConfig
	mounted      bool
}

type ActionResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Tone    string `json:"tone,omitempty"`
}

type PageConfig struct {
	Slug        string
	Title       string
	Description string
	Handler     PageHandler
}

type ResourceConfig struct {
	Name         string
	Title        string
	Description  string
	Columns      []Column
	RowActions   []RowAction
	EmptyMessage string
	ListHandler  ResourceListHandler
}

type Column struct {
	Key      string
	Title    string
	Renderer func(row map[string]interface{}) any
}

type RowAction struct {
	Label   string
	Variant string
	Href    func(row map[string]interface{}) string
}

type PageData struct {
	Title       string
	Description string
	Notices     []Notice
	Stats       []Stat
	Sections    []Section
}

type Notice struct {
	Message string
	Tone    string
}

type Stat struct {
	Label string
	Value any
	Tone  string
}

type Section struct {
	Title       string
	Description string
	ContentHTML template.HTML
}

type navItem struct {
	Title string
	Href  string
	Kind  string
}

type dashboardCard struct {
	Title       string
	Description string
	Href        string
	Kind        string
}

type resourceRowAction struct {
	Label   string
	Href    string
	Variant string
}

type resourceRowView struct {
	Cells   []any
	Actions []resourceRowAction
}

type layoutViewData struct {
	LayoutTitle  string
	LayoutSub    string
	FrameworkURL string
	HeadHTML     template.HTML
	CurrentPath  string
	Nav          []navItem
	PageTitle    string
	Subtitle     string
	Content      contentViewData
	Now          string
}

type contentViewData struct {
	Cards        []dashboardCard
	Notices      []Notice
	Stats        []Stat
	Sections     []Section
	Columns      []string
	Rows         []resourceRowView
	EmptyMessage string
}

func New(group types.GroupBuilder) *Builder {
	basePath := normalizeBasePath(group.BasePath())
	tmpl := template.Must(template.New("admin").Funcs(template.FuncMap{
		"isHTML": func(v any) bool {
			_, ok := v.(template.HTML)
			return ok
		},
		"badgeClass": func(tone string) string {
			switch strings.ToLower(strings.TrimSpace(tone)) {
			case "danger", "error":
				return "bg-rose-100 text-rose-700 ring-1 ring-inset ring-rose-200"
			case "success", "ok":
				return "bg-emerald-100 text-emerald-700 ring-1 ring-inset ring-emerald-200"
			case "warning", "warn":
				return "bg-amber-100 text-amber-700 ring-1 ring-inset ring-amber-200"
			default:
				return "bg-slate-100 text-slate-700 ring-1 ring-inset ring-slate-200"
			}
		},
		"buttonClass": func(variant string) string {
			base := "inline-flex items-center rounded-lg px-3 py-2 text-sm font-medium transition focus:outline-none focus:ring-2 focus:ring-offset-2"
			switch variant {
			case "primary":
				return base + " bg-indigo-600 text-white hover:bg-indigo-500 focus:ring-indigo-500"
			case "danger":
				return base + " bg-rose-600 text-white hover:bg-rose-500 focus:ring-rose-500"
			default:
				return base + " bg-white text-slate-700 ring-1 ring-inset ring-slate-300 hover:bg-slate-50 focus:ring-slate-400"
			}
		},
	}).Parse(layoutTemplate))

	return &Builder{
		group:        group,
		basePath:     basePath,
		title:        "Service Admin",
		subtitle:     "Universal admin panel powered by sai-service",
		frameworkURL: basePath + "/assets/tailwind.js",
		headHTML: template.HTML(
			`<link rel="preload" href="` + basePath + `/assets/fonts/inter-cyrillic.woff2" as="font" type="font/woff2" crossorigin>` +
				`<link rel="preload" href="` + basePath + `/assets/fonts/inter-latin.woff2" as="font" type="font/woff2" crossorigin>` +
				`<link rel="stylesheet" href="` + basePath + `/assets/fonts/inter.css">`,
		),
		tmpl:      tmpl,
		pages:     make([]*PageConfig, 0),
		resources: make([]*ResourceConfig, 0),
	}
}

func (b *Builder) WithTitle(title string) *Builder {
	if strings.TrimSpace(title) != "" {
		b.title = strings.TrimSpace(title)
	}
	return b
}

func (b *Builder) WithSubtitle(subtitle string) *Builder {
	b.subtitle = strings.TrimSpace(subtitle)
	return b
}

func (b *Builder) WithFrameworkURL(url string) *Builder {
	if strings.TrimSpace(url) != "" {
		b.frameworkURL = strings.TrimSpace(url)
	}
	return b
}

func (b *Builder) WithHeadHTML(html string) *Builder {
	b.headHTML = template.HTML(html)
	return b
}

func (b *Builder) WithAuthProvider(provider string) *Builder {
	b.authProvider = strings.TrimSpace(provider)
	return b
}

func (b *Builder) WithHomePage(title, description string, handler PageHandler) *Builder {
	return b.WithHomePageConfig(PageConfig{
		Title:       title,
		Description: description,
		Handler:     handler,
	})
}

func (b *Builder) WithHomePageConfig(cfg PageConfig) *Builder {
	b.homePage = &PageConfig{
		Slug:        "home",
		Title:       strings.TrimSpace(cfg.Title),
		Description: strings.TrimSpace(cfg.Description),
		Handler:     cfg.Handler,
	}
	return b
}

func (b *Builder) Page(slug, title string, handler PageHandler) *Builder {
	return b.PageWithConfig(slug, PageConfig{
		Title:   title,
		Handler: handler,
	})
}

func (b *Builder) PageWithConfig(slug string, cfg PageConfig) *Builder {
	cfg.Slug = normalizeSlug(slug)
	b.pages = append(b.pages, &PageConfig{
		Slug:        cfg.Slug,
		Title:       strings.TrimSpace(cfg.Title),
		Description: strings.TrimSpace(cfg.Description),
		Handler:     cfg.Handler,
	})
	return b
}

func (b *Builder) Resource(name string, cfg ResourceConfig) *Builder {
	cfg.Name = normalizeSlug(name)
	if strings.TrimSpace(cfg.Title) == "" {
		cfg.Title = humanize(cfg.Name)
	}
	if cfg.EmptyMessage == "" {
		cfg.EmptyMessage = "No records yet."
	}
	b.resources = append(b.resources, &cfg)
	return b
}

func (b *Builder) Mount() *Builder {
	if b.mounted {
		return b
	}

	group := b.group.
		WithoutMiddlewares("cache").
		WithTimeout(10 * time.Second)
	if b.authProvider != "" {
		group = group.WithAuthProvider(b.authProvider)
	} else {
		group = group.WithoutMiddlewares("auth")
	}

	group.GET("", b.handleIndex)
	b.mountAssets(group)

	for _, page := range b.pages {
		pageCfg := page
		group.GET("/pages/"+page.Slug, func(ctx *types.RequestCtx) {
			b.handlePage(ctx, pageCfg)
		})
	}

	for _, resource := range b.resources {
		resourceCfg := resource
		group.GET("/resources/"+resource.Name, func(ctx *types.RequestCtx) {
			b.handleResource(ctx, resourceCfg)
		})
	}

	b.mounted = true
	return b
}

func (b *Builder) handleIndex(ctx *types.RequestCtx) {
	if b.homePage != nil {
		b.handlePageAtPath(ctx, b.basePath, b.homePage)
		return
	}

	cards := make([]dashboardCard, 0, len(b.pages)+len(b.resources))
	for _, page := range b.pages {
		cards = append(cards, dashboardCard{
			Title:       page.Title,
			Description: fallback(page.Description, "Custom admin page"),
			Href:        b.basePath + "/pages/" + page.Slug,
			Kind:        "Page",
		})
	}
	for _, resource := range b.resources {
		cards = append(cards, dashboardCard{
			Title:       resource.Title,
			Description: fallback(resource.Description, "Resource browser"),
			Href:        b.basePath + "/resources/" + resource.Name,
			Kind:        "Resource",
		})
	}

	b.render(ctx, b.basePath, "Overview", "Choose a page or resource to manage.", contentViewData{Cards: cards})
}

func (b *Builder) handlePage(ctx *types.RequestCtx, page *PageConfig) {
	b.handlePageAtPath(ctx, b.basePath+"/pages/"+page.Slug, page)
}

func (b *Builder) handlePageAtPath(ctx *types.RequestCtx, currentPath string, page *PageConfig) {
	if page.Handler == nil {
		ctx.Error(fmt.Errorf("admin page %q has no handler", page.Slug), fasthttp.StatusInternalServerError)
		return
	}

	data, err := page.Handler(ctx)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	title := page.Title
	subtitle := page.Description
	if data != nil {
		if strings.TrimSpace(data.Title) != "" {
			title = strings.TrimSpace(data.Title)
		}
		if strings.TrimSpace(data.Description) != "" {
			subtitle = strings.TrimSpace(data.Description)
		}
	}

	view := contentViewData{}
	if data != nil {
		view.Notices = data.Notices
		view.Stats = data.Stats
		view.Sections = data.Sections
	}

	b.render(ctx, currentPath, title, subtitle, view)
}

func (b *Builder) handleResource(ctx *types.RequestCtx, resource *ResourceConfig) {
	if resource.ListHandler == nil {
		ctx.Error(fmt.Errorf("admin resource %q has no list handler", resource.Name), fasthttp.StatusInternalServerError)
		return
	}

	rows, err := resource.ListHandler(ctx)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	columns := resource.Columns
	if len(columns) == 0 {
		columns = inferColumns(rows)
	}

	view := contentViewData{
		Columns:      make([]string, 0, len(columns)),
		Rows:         make([]resourceRowView, 0, len(rows)),
		EmptyMessage: resource.EmptyMessage,
	}

	for _, column := range columns {
		view.Columns = append(view.Columns, fallback(column.Title, humanize(column.Key)))
	}

	for _, row := range rows {
		rowView := resourceRowView{
			Cells:   make([]any, 0, len(columns)),
			Actions: make([]resourceRowAction, 0, len(resource.RowActions)),
		}

		for _, column := range columns {
			var value any
			if column.Renderer != nil {
				value = column.Renderer(row)
			} else {
				value = lookupValue(row, column.Key)
			}
			rowView.Cells = append(rowView.Cells, formatValue(value))
		}

		for _, action := range resource.RowActions {
			if action.Href == nil {
				continue
			}
			href := strings.TrimSpace(action.Href(row))
			if href == "" {
				continue
			}
			rowView.Actions = append(rowView.Actions, resourceRowAction{
				Label:   fallback(action.Label, "Open"),
				Href:    href,
				Variant: fallback(action.Variant, "secondary"),
			})
		}

		view.Rows = append(view.Rows, rowView)
	}

	b.render(ctx, b.basePath+"/resources/"+resource.Name, resource.Title, resource.Description, view)
}

func (b *Builder) render(ctx *types.RequestCtx, currentPath, pageTitle, subtitle string, content contentViewData) {
	if IsFragmentRequest(ctx) {
		b.renderContent(ctx, content)
		return
	}

	data := layoutViewData{
		LayoutTitle:  b.title,
		LayoutSub:    b.subtitle,
		FrameworkURL: b.frameworkURL,
		HeadHTML:     b.headHTML,
		CurrentPath:  currentPath,
		Nav:          b.navItems(),
		PageTitle:    pageTitle,
		Subtitle:     subtitle,
		Content:      content,
		Now:          time.Now().Format("2006-01-02 15:04:05"),
	}

	var buf strings.Builder
	if err := b.tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	_, _ = ctx.Success([]byte(buf.String()), []byte("text/html; charset=UTF-8"))
}

func (b *Builder) renderContent(ctx *types.RequestCtx, content contentViewData) {
	var buf strings.Builder
	if err := b.tmpl.ExecuteTemplate(&buf, "content", content); err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	_, _ = ctx.Success([]byte(buf.String()), []byte("text/html; charset=UTF-8"))
}

func (b *Builder) navItems() []navItem {
	homeTitle := "Overview"
	if b.homePage != nil && strings.TrimSpace(b.homePage.Title) != "" {
		homeTitle = b.homePage.Title
	}
	items := []navItem{{Title: homeTitle, Href: b.basePath, Kind: "Home"}}

	for _, page := range b.pages {
		items = append(items, navItem{
			Title: page.Title,
			Href:  b.basePath + "/pages/" + page.Slug,
			Kind:  "Page",
		})
	}

	for _, resource := range b.resources {
		items = append(items, navItem{
			Title: resource.Title,
			Href:  b.basePath + "/resources/" + resource.Name,
			Kind:  "Resource",
		})
	}

	return items
}

func HTML(value string) template.HTML {
	return template.HTML(value)
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ToneFromCount(count int, nonZeroTone, zeroTone string) string {
	if count > 0 {
		return strings.TrimSpace(nonZeroTone)
	}
	return strings.TrimSpace(zeroTone)
}

func SetFlash(ctx *types.RequestCtx, path, message, tone string) {
	message = strings.TrimSpace(message)
	tone = strings.TrimSpace(tone)
	if message == "" {
		return
	}

	var cookie fasthttp.Cookie
	cookie.SetKey("admin_flash")
	cookie.SetValue(base64.StdEncoding.EncodeToString([]byte(tone + "|" + message)))
	cookie.SetPath(normalizeBasePath(path))
	cookie.SetHTTPOnly(true)
	ctx.Response.Header.SetCookie(&cookie)
}

func ReadFlash(ctx *types.RequestCtx, path string) []Notice {
	raw := strings.TrimSpace(string(ctx.Request.Header.Cookie("admin_flash")))
	if raw == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		ClearFlash(ctx, path)
		return nil
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		ClearFlash(ctx, path)
		return nil
	}

	ClearFlash(ctx, path)
	return []Notice{{
		Tone:    strings.TrimSpace(parts[0]),
		Message: strings.TrimSpace(parts[1]),
	}}
}

func ClearFlash(ctx *types.RequestCtx, path string) {
	var cookie fasthttp.Cookie
	cookie.SetKey("admin_flash")
	cookie.SetPath(normalizeBasePath(path))
	cookie.SetExpire(time.Unix(0, 0))
	cookie.SetMaxAge(-1)
	ctx.Response.Header.SetCookie(&cookie)
}

func RedirectWithFlash(ctx *types.RequestCtx, path, message string, err error) {
	if err != nil {
		SetFlash(ctx, path, err.Error(), "danger")
	} else if strings.TrimSpace(message) != "" {
		SetFlash(ctx, path, message, "success")
	}
	ctx.Redirect(normalizeBasePath(path), fasthttp.StatusSeeOther)
}

func IsActionRequest(ctx *types.RequestCtx) bool {
	return strings.EqualFold(strings.TrimSpace(string(ctx.Request.Header.Peek("X-Requested-With"))), "fetch")
}

func IsFragmentRequest(ctx *types.RequestCtx) bool {
	return strings.EqualFold(strings.TrimSpace(string(ctx.Request.Header.Peek("X-SAI-Admin-Fragment"))), "1")
}

func WriteActionJSON(ctx *types.RequestCtx, message string, err error) {
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		_, _ = ctx.SuccessJSON(ActionResponse{
			OK:    false,
			Error: err.Error(),
			Tone:  "danger",
		})
		return
	}

	_, _ = ctx.SuccessJSON(ActionResponse{
		OK:      true,
		Message: strings.TrimSpace(message),
		Tone:    "success",
	})
}

func normalizeBasePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/admin"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/admin"
	}
	return path
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func inferColumns(rows []map[string]interface{}) []Column {
	if len(rows) == 0 {
		return nil
	}

	keys := make([]string, 0, len(rows[0]))
	for key := range rows[0] {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	columns := make([]Column, 0, len(keys))
	for _, key := range keys {
		columns = append(columns, Column{
			Key:   key,
			Title: humanize(key),
		})
	}
	return columns
}

func lookupValue(row map[string]interface{}, key string) any {
	if key == "" {
		return ""
	}

	parts := strings.Split(key, ".")
	var current any = row
	for _, part := range parts {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = asMap[part]
		if !ok {
			return nil
		}
	}

	return current
}

func formatValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return ""
	case template.HTML:
		return typed
	case string:
		return typed
	case time.Time:
		return typed.Format(time.RFC3339)
	case fmt.Stringer:
		return typed.String()
	default:
		bytes, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		if len(bytes) > 128 {
			return string(bytes[:125]) + "..."
		}
		return string(bytes)
	}
}

func normalizeSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Trim(value, "-/")
	if value == "" {
		return "item"
	}
	return value
}

func humanize(value string) string {
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.TrimSpace(value)
	if value == "" {
		return "Item"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

const layoutTemplate = `
{{define "layout"}}
<!doctype html>
<html lang="en" class="h-full bg-slate-50">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.PageTitle}} | {{.LayoutTitle}}</title>
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Crect width='64' height='64' rx='14' fill='%232563eb'/%3E%3Ctext x='32' y='43' text-anchor='middle' font-family='Inter,Arial,sans-serif' font-size='34' font-weight='700' fill='white'%3ES%3C/text%3E%3C/svg%3E">
  {{if .FrameworkURL}}<script src="{{.FrameworkURL}}"></script>{{end}}
  <script>
    tailwind.config = {
      theme: {
        extend: {
          colors: {
            brand: {
              50: '#eef2ff',
              100: '#e0e7ff',
              500: '#6366f1',
              600: '#4f46e5',
              700: '#4338ca'
            }
          },
          boxShadow: {
            panel: '0 18px 50px rgba(15, 23, 42, 0.08)'
          }
        }
      }
    }
  </script>
  <style>
    body { font-family: 'Inter', sans-serif; }
  </style>
  {{.HeadHTML}}
</head>
<body class="h-full bg-slate-50 text-slate-900">
  <div class="min-h-full">
    <div class="lg:hidden sticky top-0 z-40 border-b border-slate-200 bg-white/95 backdrop-blur">
      <div class="flex items-center justify-between px-4 py-3">
        <div>
          <div class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">sai-service admin</div>
          <div class="text-sm font-semibold text-slate-900">{{.LayoutTitle}}</div>
        </div>
        <button id="admin-sidebar-open" type="button" class="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-700 shadow-sm">
          <span class="sr-only">Open menu</span>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M3 5a1 1 0 011-1h12a1 1 0 110 2H4A1 1 0 013 5zm0 5a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm1 4a1 1 0 100 2h12a1 1 0 100-2H4z" clip-rule="evenodd"/>
          </svg>
        </button>
      </div>
    </div>
    <div class="flex min-h-screen w-full flex-col lg:flex-row">
      <div id="admin-sidebar-backdrop" class="fixed inset-0 z-40 hidden bg-slate-950/50 lg:hidden"></div>
      <aside id="admin-sidebar" class="fixed inset-y-0 left-0 z-50 w-[88vw] max-w-80 -translate-x-full overflow-y-auto border-r border-slate-200 bg-slate-900 px-6 py-6 text-slate-100 shadow-2xl transition-transform duration-200 ease-out lg:sticky lg:top-0 lg:z-auto lg:block lg:h-screen lg:w-80 lg:max-w-none lg:translate-x-0 lg:self-start lg:overflow-hidden lg:shadow-none lg:px-7 lg:py-8">
        <div class="mb-4 flex items-center justify-between lg:hidden">
          <div class="text-sm font-semibold text-white">{{.LayoutTitle}}</div>
          <button id="admin-sidebar-close" type="button" class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-white/10 bg-white/5 text-slate-100">
            <span class="sr-only">Close menu</span>
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/>
            </svg>
          </button>
        </div>
        <div class="mb-8">
          <div class="mb-3 inline-flex rounded-full bg-white/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-slate-200">
            sai-service admin
          </div>
          <h1 class="text-2xl font-semibold tracking-tight">{{.LayoutTitle}}</h1>
          {{if .LayoutSub}}<p class="mt-3 text-sm leading-6 text-slate-300">{{.LayoutSub}}</p>{{end}}
        </div>
        <nav class="space-y-2">
          {{range .Nav}}
          <a href="{{.Href}}" class="block rounded-2xl px-4 py-3 transition {{if eq $.CurrentPath .Href}}bg-white text-slate-900 shadow-panel{{else}}bg-white/5 text-slate-100 hover:bg-white/10{{end}}">
            <div class="text-[11px] font-semibold uppercase tracking-[0.18em] {{if eq $.CurrentPath .Href}}text-slate-500{{else}}text-slate-300{{end}}">{{.Kind}}</div>
            <div class="mt-1 text-sm font-medium">{{.Title}}</div>
          </a>
          {{end}}
        </nav>
      </aside>
      <main class="flex-1 px-4 py-5 sm:px-6 sm:py-6 lg:px-10 lg:py-8">
        <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="inline-flex items-center gap-3 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-panel">
            <div class="flex h-10 w-10 items-center justify-center rounded-full bg-brand-100 text-sm font-semibold text-brand-700">
              AD
            </div>
            <div>
              <div class="text-sm font-semibold text-slate-900">Admin Session</div>
              <div class="text-xs text-slate-500">Universal sai-service panel</div>
            </div>
          </div>
          <div class="inline-flex items-center rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-600 shadow-panel">
            {{.LayoutTitle}}
          </div>
        </div>

        <section class="rounded-[28px] border border-slate-200 bg-gradient-to-br from-slate-900 via-slate-800 to-indigo-900 px-6 py-6 text-white shadow-panel sm:px-8">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <div class="mb-3 inline-flex rounded-full bg-white/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-indigo-100">
                Dashboard
              </div>
              <h2 class="text-3xl font-semibold tracking-tight sm:text-4xl">{{.PageTitle}}</h2>
              {{if .Subtitle}}<p class="mt-3 max-w-3xl text-sm leading-6 text-slate-200 sm:text-base">{{.Subtitle}}</p>{{end}}
            </div>
            <div class="rounded-2xl bg-white/10 px-4 py-3 text-sm text-slate-100">
              Rendered at {{.Now}}
            </div>
          </div>
        </section>

        <div id="admin-page-content" class="mt-6">
          {{template "content" .Content}}
        </div>
      </main>
    </div>
  </div>
  <script>
    (function () {
      if (!window.__saiAdmin) {
        window.__saiAdmin = {};
      }

      window.__saiAdmin.showToast = function (message, tone) {
        var container = document.getElementById('admin-toasts-live');
        if (!container) {
          container = document.createElement('div');
          container.id = 'admin-toasts-live';
          container.className = 'pointer-events-none fixed right-4 top-4 z-50 flex w-[min(92vw,24rem)] flex-col gap-3';
          document.body.appendChild(container);
        }

        var toneClass = 'bg-slate-100 text-slate-700 ring-1 ring-inset ring-slate-200';
        var title = 'Done';
        if (tone === 'danger' || tone === 'error') {
          toneClass = 'bg-rose-100 text-rose-700 ring-1 ring-inset ring-rose-200';
          title = 'Error';
        } else if (tone === 'success' || tone === 'ok') {
          toneClass = 'bg-emerald-100 text-emerald-700 ring-1 ring-inset ring-emerald-200';
        } else if (tone === 'warning' || tone === 'warn') {
          toneClass = 'bg-amber-100 text-amber-700 ring-1 ring-inset ring-amber-200';
        }

        var toast = document.createElement('div');
        toast.className = 'pointer-events-auto rounded-2xl border px-4 py-3 shadow-panel ' + toneClass;
        toast.innerHTML = '<div class="flex items-start gap-3"><div class="mt-0.5 text-sm font-semibold">' + title + '</div><div class="flex-1 text-sm leading-6"></div><button type="button" class="text-sm font-semibold opacity-70 hover:opacity-100">x</button></div>';
        toast.querySelector('.flex-1').textContent = message;
        toast.querySelector('button').addEventListener('click', function () { toast.remove(); });
        container.appendChild(toast);

        setTimeout(function () {
          toast.style.transition = 'opacity .25s ease, transform .25s ease';
          toast.style.opacity = '0';
          toast.style.transform = 'translateY(-6px)';
          setTimeout(function () { toast.remove(); }, 260);
        }, 2800);
      };

      if (!window.__saiAdmin.bindAjaxForms) {
        window.__saiAdmin.bindAjaxForms = true;

        async function refreshContent() {
          var target = document.getElementById('admin-page-content');
          if (!target) return;

          var response = await fetch(window.location.pathname + window.location.search, {
            headers: {
              'X-Requested-With': 'fetch',
              'X-SAI-Admin-Fragment': '1'
            }
          });
          if (!response.ok) throw new Error('Failed to refresh admin content');
          target.innerHTML = await response.text();
        }

        async function submitAdminForm(form) {
          var submitter = form.querySelector('button[type="submit"]');
          var originalText = submitter ? submitter.textContent : '';
          var confirmText = form.getAttribute('data-admin-confirm');

          if (confirmText && !window.confirm(confirmText)) {
            return;
          }

          try {
            if (submitter) {
              submitter.disabled = true;
              submitter.textContent = 'Working...';
            }

            var response = await fetch(form.action, {
              method: form.method || 'POST',
              body: new FormData(form),
              headers: { 'X-Requested-With': 'fetch' }
            });

            var payload = await response.json();
            if (!response.ok || !payload.ok) {
              throw new Error(payload.error || payload.message || 'Request failed');
            }

            await refreshContent();
            if (payload.message) {
              window.__saiAdmin.showToast(payload.message, payload.tone || 'success');
            }
          } catch (error) {
            window.__saiAdmin.showToast(error.message || 'Request failed', 'danger');
          } finally {
            if (submitter) {
              submitter.disabled = false;
              submitter.textContent = originalText;
            }
          }
        }

        document.addEventListener('submit', function (event) {
          var form = event.target;
          if (!(form instanceof HTMLFormElement)) return;
          if (form.getAttribute('data-admin-ajax') !== 'true') return;
          event.preventDefault();
          submitAdminForm(form);
        });
      }

      var sidebar = document.getElementById('admin-sidebar');
      var backdrop = document.getElementById('admin-sidebar-backdrop');
      var openBtn = document.getElementById('admin-sidebar-open');
      var closeBtn = document.getElementById('admin-sidebar-close');
      if (!sidebar || !backdrop || !openBtn || !closeBtn) return;

      function openSidebar() {
        sidebar.classList.remove('-translate-x-full');
        backdrop.classList.remove('hidden');
        document.body.classList.add('overflow-hidden');
      }

      function closeSidebar() {
        sidebar.classList.add('-translate-x-full');
        backdrop.classList.add('hidden');
        document.body.classList.remove('overflow-hidden');
      }

      openBtn.addEventListener('click', openSidebar);
      closeBtn.addEventListener('click', closeSidebar);
      backdrop.addEventListener('click', closeSidebar);
      window.addEventListener('resize', function () {
        if (window.innerWidth >= 1024) {
          closeSidebar();
        }
      });
    })();
  </script>
</body>
</html>
{{end}}

{{define "content"}}
  {{if .Cards}}
  <section class="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
    {{range .Cards}}
    <a href="{{.Href}}" class="group rounded-[24px] border border-slate-200 bg-white p-6 shadow-panel transition hover:-translate-y-0.5 hover:border-brand-200 hover:shadow-xl">
      <div class="mb-4 inline-flex rounded-full bg-brand-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-brand-700">{{.Kind}}</div>
      <h3 class="text-xl font-semibold tracking-tight text-slate-900">{{.Title}}</h3>
      <p class="mt-3 text-sm leading-6 text-slate-600">{{.Description}}</p>
      <div class="mt-5 text-sm font-medium text-brand-600">Open page</div>
    </a>
    {{end}}
  </section>
  {{end}}

  {{if .Notices}}
  <div id="admin-toasts" class="pointer-events-none fixed right-4 top-4 z-50 flex w-[min(92vw,24rem)] flex-col gap-3">
    {{range .Notices}}
    <div class="pointer-events-auto rounded-2xl border px-4 py-3 shadow-panel {{badgeClass .Tone}}">
      <div class="flex items-start gap-3">
        <div class="mt-0.5 text-sm font-semibold">{{if eq .Tone "danger"}}Error{{else}}Done{{end}}</div>
        <div class="flex-1 text-sm leading-6">{{.Message}}</div>
        <button type="button" class="toast-close text-sm font-semibold opacity-70 hover:opacity-100">x</button>
      </div>
    </div>
    {{end}}
  </div>
  <script>
    setTimeout(function () {
      document.querySelectorAll('#admin-toasts > div').forEach(function (el) {
        el.style.transition = 'opacity .25s ease, transform .25s ease';
        el.style.opacity = '0';
        el.style.transform = 'translateY(-6px)';
        setTimeout(function () { el.remove(); }, 260);
      });
    }, 2800);
    document.querySelectorAll('.toast-close').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var toast = btn.closest('div.pointer-events-auto');
        if (toast) toast.remove();
      });
    });
  </script>
  {{end}}

  {{if .Stats}}
  <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
    {{range .Stats}}
    <div class="rounded-[24px] border border-slate-200 bg-white p-5 shadow-panel">
      <div class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">{{.Label}}</div>
      <div class="mt-3 text-3xl font-semibold tracking-tight text-slate-900">{{.Value}}</div>
      {{if .Tone}}<div class="mt-4 inline-flex rounded-full px-2.5 py-1 text-xs font-medium {{badgeClass .Tone}}">{{.Tone}}</div>{{end}}
    </div>
    {{end}}
  </section>
  {{end}}

  {{if .Sections}}
  <section class="mt-6 space-y-5">
    {{range .Sections}}
    <article class="rounded-[24px] border border-slate-200 bg-white p-6 shadow-panel">
      <h3 class="text-xl font-semibold tracking-tight text-slate-900">{{.Title}}</h3>
      {{if .Description}}<p class="mt-2 text-sm leading-6 text-slate-600">{{.Description}}</p>{{end}}
      <div class="prose prose-slate mt-5 max-w-none">{{.ContentHTML}}</div>
    </article>
    {{end}}
  </section>
  {{end}}

  {{if .Columns}}
    {{if .Rows}}
    <section class="overflow-hidden rounded-[24px] border border-slate-200 bg-white shadow-panel">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-slate-200">
          <thead class="bg-slate-50">
            <tr>
              {{range .Columns}}
              <th class="px-5 py-4 text-left text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">{{.}}</th>
              {{end}}
              <th class="px-5 py-4 text-left text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            {{range .Rows}}
            <tr class="align-top">
              {{range .Cells}}
              <td class="px-5 py-4 text-sm text-slate-700">{{if isHTML .}}{{.}}{{else}}{{.}}{{end}}</td>
              {{end}}
              <td class="px-5 py-4">
                {{if .Actions}}
                <div class="flex flex-wrap gap-2">
                  {{range .Actions}}
                  <a href="{{.Href}}" class="{{buttonClass .Variant}}">{{.Label}}</a>
                  {{end}}
                </div>
                {{end}}
              </td>
            </tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </section>
    {{else}}
    <section class="rounded-[24px] border border-dashed border-slate-300 bg-white p-10 text-center text-sm text-slate-500 shadow-panel">
      {{.EmptyMessage}}
    </section>
    {{end}}
  {{end}}
{{end}}
`
