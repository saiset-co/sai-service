package types

import (
	"reflect"
	"time"
)

type DocumentationManager interface {
	LifecycleManager
	RegisterRoutes(router HTTPRouter)
	AddRoute(*RouteConfig) error
	Generate() error
	GetSpec() *OpenAPISpec
}

type OpenAPISpec struct {
	OpenAPI    string                    `json:"openapi"`
	Info       SpecInfo                  `json:"info"`
	Servers    []SpecServer              `json:"servers"`
	Paths      map[string]*RoutePathItem `json:"paths"`
	Components *SpecComponents           `json:"components"`
	Tags       []string                  `json:"tags"`
}

type SpecInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type SpecServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type SpecComponents struct {
	Schemas         map[string]*RouteSchema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*RouteSecurityScheme `json:"securitySchemes,omitempty"`
}

type RoutePathItem struct {
	Get    *RouteOperation `json:"get,omitempty"`
	Post   *RouteOperation `json:"post,omitempty"`
	Put    *RouteOperation `json:"put,omitempty"`
	Delete *RouteOperation `json:"delete,omitempty"`
	Patch  *RouteOperation `json:"patch,omitempty"`
}

type RouteOperation struct {
	Summary     string                    `json:"summary,omitempty"`
	Description string                    `json:"description,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Parameters  []RouteParameter          `json:"parameters,omitempty"`
	RequestBody *RouteRequestBody         `json:"requestBody,omitempty"`
	Responses   map[string]*RouteResponse `json:"responses"`
	Security    []map[string][]string     `json:"security,omitempty"`
}

type RouteParameter struct {
	Name        string       `json:"name"`
	In          string       `json:"in"`
	Required    bool         `json:"required,omitempty"`
	Description string       `json:"description,omitempty"`
	Schema      *RouteSchema `json:"schema,omitempty"`
	Example     interface{}  `json:"example,omitempty"`
}

type RouteRequestBody struct {
	Description string                     `json:"description,omitempty"`
	Content     map[string]*RouteMediaType `json:"content"`
	Required    bool                       `json:"required,omitempty"`
}

type RouteMediaType struct {
	Schema  *RouteSchema `json:"schema,omitempty"`
	Example interface{}  `json:"example,omitempty"`
}

type RouteResponse struct {
	Description string                     `json:"description"`
	Content     map[string]*RouteMediaType `json:"content,omitempty"`
}

type RouteSchema struct {
	Type        string                  `json:"type,omitempty"`
	Format      string                  `json:"format,omitempty"`
	Description string                  `json:"description,omitempty"`
	Properties  map[string]*RouteSchema `json:"properties,omitempty"`
	Required    []string                `json:"required,omitempty"`
	Items       *RouteSchema            `json:"items,omitempty"`
	Example     interface{}             `json:"example,omitempty"`
	Enum        []interface{}           `json:"enum,omitempty"`
	Minimum     *float64                `json:"minimum,omitempty"`
	Maximum     *float64                `json:"maximum,omitempty"`
	MinLength   *int                    `json:"minLength,omitempty"`
	MaxLength   *int                    `json:"maxLength,omitempty"`
}

type RouteSecurityScheme struct {
	Type         string `json:"type"`
	Description  string `json:"description,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
}

type RouteDocumentation struct {
	Method       string
	Path         string
	Title        string
	Description  string
	Tags         []string
	RequestType  reflect.Type
	ResponseType reflect.Type
	CreatedAt    time.Time
}

type DocConfig struct {
	Path            string
	Method          string
	DocTitle        string
	DocDescription  string
	DocTag          string
	DocRequestType  reflect.Type
	DocResponseType reflect.Type
}
