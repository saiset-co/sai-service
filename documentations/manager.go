package documentations

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type DocumentationManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          types.ConfigManager
	logger          types.Logger
	health          types.HealthManager
	router          types.HTTPRouter
	mu              sync.RWMutex
	spec            *types.OpenAPISpec
	state           atomic.Value
	shutdownTimeout time.Duration
	generateTimeout time.Duration
}

func NewDocumentationManager(config types.ConfigManager, logger types.Logger, health types.HealthManager, router types.HTTPRouter) (types.DocumentationManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	dm := &DocumentationManager{
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		health:          health,
		router:          router,
		shutdownTimeout: 10 * time.Second,
		generateTimeout: 30 * time.Second,
	}

	dm.state.Store(StateStopped)

	return dm, nil
}

func (dm *DocumentationManager) Start() error {
	if !dm.transitionState(StateStopped, StateStarting) {
		dm.logger.Warn("Documentation manager is already running")
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if dm.getState() == StateStarting {
			dm.setState(StateRunning)
		}
	}()

	ctx, cancel := context.WithTimeout(dm.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			dm.registerRoutes()
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		dm.setState(StateStopped)
		return types.WrapError(err, "failed to start documentation manager")
	}

	dm.logger.Info("Documentation manager started")
	return nil
}

func (dm *DocumentationManager) Stop() error {
	if !dm.transitionState(StateRunning, StateStopping) {
		dm.logger.Warn("Documentation manager is not running")
		return types.ErrServerNotRunning
	}

	defer func() {
		dm.setState(StateStopped)
		dm.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), dm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		dm.mu.Lock()
		defer dm.mu.Unlock()
		dm.spec = nil
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			dm.logger.Warn("Documentation manager stop timeout, some components may not have stopped gracefully")
		default:
			dm.logger.Error("Error during documentation manager shutdown", zap.Error(err))
		}
	} else {
		dm.logger.Info("Documentation manager stopped gracefully")
	}

	return nil
}

func (dm *DocumentationManager) IsRunning() bool {
	return dm.getState() == StateRunning
}

func (dm *DocumentationManager) getState() State {
	return dm.state.Load().(State)
}

func (dm *DocumentationManager) setState(newState State) bool {
	currentState := dm.getState()
	return dm.state.CompareAndSwap(currentState, newState)
}

func (dm *DocumentationManager) transitionState(from, to State) bool {
	return dm.state.CompareAndSwap(from, to)
}

func (dm *DocumentationManager) registerRoutes() {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"cache"},
		Doc:                 nil,
	}

	dm.router.Add("GET", dm.config.GetConfig().Docs.Path, dm.handleDocs, config)
	dm.router.Add("GET", "/openapi.json", dm.handleOpenAPIJSON, config)
}

func (dm *DocumentationManager) generate() error {
	if !dm.IsRunning() {
		return types.ErrActionNotInitialized
	}

	generateCtx, cancel := context.WithTimeout(dm.ctx, dm.generateTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(generateCtx)

	var routes map[string]*types.RouteInfo
	var config *types.ServiceConfig
	var spec *types.OpenAPISpec

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			dm.mu.RLock()
			routes = make(map[string]*types.RouteInfo)
			for k, v := range dm.router.GetAllRoutes() {
				routes[k] = v
			}
			dm.mu.RUnlock()
			return nil
		}
	})

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			config = dm.config.GetConfig()
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-generateCtx.Done():
			return types.NewErrorf("documentation generation timeout")
		default:
			return types.WrapError(err, "failed to prepare generation data")
		}
	}

	g, gCtx = errgroup.WithContext(generateCtx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			spec = &types.OpenAPISpec{
				OpenAPI: "3.0.3",
				Info: types.SpecInfo{
					Title:       config.Name,
					Version:     config.Version,
					Description: fmt.Sprintf("%s API documentation", config.Name),
				},
				Servers: dm.generateServers(),
				Paths:   make(map[string]*types.RoutePathItem),
				Tags:    dm.generateTags(routes),
				Components: &types.SpecComponents{
					Schemas:         dm.generateSchemas(routes),
					SecuritySchemes: dm.generateSecuritySchemes(),
				},
			}
			return nil
		}
	})

	var processedPaths map[string]*types.RoutePathItem

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			processedPaths = make(map[string]*types.RoutePathItem)

			pathG, pathGCtx := errgroup.WithContext(gCtx)
			pathMu := sync.Mutex{}

			for _, route := range routes {
				if route.Config.Doc == nil {
					continue
				}

				r := route
				pathG.Go(func() error {
					select {
					case <-pathGCtx.Done():
						return pathGCtx.Err()
					default:
						pathItem := dm.generatePathItem(r)
						if pathItem != nil {
							pathMu.Lock()
							if existingItem, ok := processedPaths[r.Config.Doc.Path]; ok {
								if pathItem.Get != nil {
									existingItem.Get = pathItem.Get
								}
								if pathItem.Post != nil {
									existingItem.Post = pathItem.Post
								}
								if pathItem.Put != nil {
									existingItem.Put = pathItem.Put
								}
								if pathItem.Delete != nil {
									existingItem.Delete = pathItem.Delete
								}
								if pathItem.Patch != nil {
									existingItem.Patch = pathItem.Patch
								}
								processedPaths[r.Config.Doc.Path] = existingItem
							} else {
								processedPaths[r.Config.Doc.Path] = pathItem
							}
							pathMu.Unlock()
						}
						return nil
					}
				})
			}

			return pathG.Wait()
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-generateCtx.Done():
			return types.NewErrorf("documentation generation timeout")
		default:
			return types.WrapError(err, "failed to generate documentation")
		}
	}

	spec.Paths = processedPaths

	dm.mu.Lock()
	dm.spec = spec
	dm.mu.Unlock()

	dm.logger.Info("OpenAPI documentation generated",
		zap.Int("routes", len(routes)),
		zap.Int("paths", len(spec.Paths)),
		zap.Int("schemas", len(spec.Components.Schemas)),
		zap.Int("tags", len(spec.Tags)))

	return nil
}

func (dm *DocumentationManager) getSpec() *types.OpenAPISpec {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.spec
}

func (dm *DocumentationManager) generateServers() []types.SpecServer {
	config := dm.config.GetConfig()

	servers := []types.SpecServer{
		{
			URL:         fmt.Sprintf("http://%s:%d", config.Server.HTTP.Host, config.Server.HTTP.Port),
			Description: "Development server",
		},
	}

	if config.Server.TLS.Enabled {
		servers = append(servers, types.SpecServer{
			URL:         fmt.Sprintf("https://%s:%d", config.Server.HTTP.Host, config.Server.HTTP.Port),
			Description: "Production server (HTTPS)",
		})
	}

	return servers
}

func (dm *DocumentationManager) generateTags(routes map[string]*types.RouteInfo) []string {
	var tags []string

	for _, route := range routes {
		if route.Config.Doc == nil {
			continue
		}

		tags = append(tags, route.Config.Doc.DocTag)
	}

	return tags
}

func (dm *DocumentationManager) generatePathItem(route *types.RouteInfo) *types.RoutePathItem {
	operation := dm.generateOperation(route)
	if operation == nil {
		return nil
	}

	pathItem := &types.RoutePathItem{}
	method := strings.ToUpper(route.Config.Doc.Method)

	switch method {
	case "GET":
		pathItem.Get = operation
	case "POST":
		pathItem.Post = operation
	case "PUT":
		pathItem.Put = operation
	case "DELETE":
		pathItem.Delete = operation
	case "PATCH":
		pathItem.Patch = operation
	default:
		return nil
	}

	return pathItem
}

func (dm *DocumentationManager) generateOperation(route *types.RouteInfo) *types.RouteOperation {
	operation := &types.RouteOperation{
		Summary:     route.Config.Doc.DocTitle,
		Description: route.Config.Doc.DocDescription,
		Tags:        []string{route.Config.Doc.DocTag},
		Parameters:  dm.generateParameters(route),
		Responses:   dm.generateResponses(route),
	}

	if route.Config.Doc.Method != "GET" && route.Config.Doc.DocRequestType != nil {
		requestBody := dm.generateRequestBody(route)
		if requestBody != nil {
			operation.RequestBody = requestBody
		}
	}

	operation.Security = []map[string][]string{
		{"ApiKeyAuth": {}},
	}

	return operation
}

func (dm *DocumentationManager) generateParameters(route *types.RouteInfo) []types.RouteParameter {
	var parameters []types.RouteParameter

	pathParams := dm.extractPathParams(route.Config.Doc.Path)
	for _, param := range pathParams {
		parameters = append(parameters, types.RouteParameter{
			Name:        param,
			In:          "path",
			Required:    true,
			Description: dm.generateParamDescription(param),
			Schema: &types.RouteSchema{
				Type: dm.inferParameterType(param),
			},
			Example: dm.generateParamExample(param),
		})
	}

	if route.Config.Doc.Method == "GET" && route.Config.Doc.DocRequestType != nil {
		queryParams := dm.generateQueryParamsFromType(route.Config.Doc.DocRequestType)
		parameters = append(parameters, queryParams...)
	}

	return parameters
}

func (dm *DocumentationManager) generateQueryParamsFromType(t reflect.Type) []types.RouteParameter {
	var parameters []types.RouteParameter

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return parameters
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		paramName := dm.getFieldName(field, jsonTag)
		required := dm.isFieldRequired(field)

		param := types.RouteParameter{
			Name:        paramName,
			In:          "query",
			Required:    required,
			Description: dm.getFieldDescription(field),
			Schema:      dm.generateSchemaFromField(field),
			Example:     dm.generateFieldExample(field),
		}

		parameters = append(parameters, param)
	}

	return parameters
}

func (dm *DocumentationManager) generateRequestBody(route *types.RouteInfo) *types.RouteRequestBody {
	if route.Config.Doc.DocRequestType == nil {
		return nil
	}

	schema := dm.generateSchemaFromType(route.Config.Doc.DocRequestType)

	return &types.RouteRequestBody{
		Description: fmt.Sprintf("Request body for %s", route.Config.Doc.DocTitle),
		Content: map[string]*types.RouteMediaType{
			"application/json": {
				Schema:  schema,
				Example: dm.generateExampleFromSchema(schema),
			},
		},
		Required: true,
	}
}

func (dm *DocumentationManager) generateResponses(route *types.RouteInfo) map[string]*types.RouteResponse {
	responses := make(map[string]*types.RouteResponse)

	if route.Config.Doc.DocResponseType != nil {
		schema := dm.generateSchemaFromType(route.Config.Doc.DocResponseType)

		example := dm.generateExampleFromSchema(schema)

		responses["200"] = &types.RouteResponse{
			Description: "Successful response",
			Content: map[string]*types.RouteMediaType{
				"application/json": {
					Schema:  schema,
					Example: example,
				},
			},
		}
	} else {
		responses["200"] = &types.RouteResponse{
			Description: "Successful response",
			Content: map[string]*types.RouteMediaType{
				"application/json": {
					Schema: &types.RouteSchema{
						Type: "object",
						Properties: map[string]*types.RouteSchema{
							"success": {Type: "boolean", Example: true},
							"message": {Type: "string", Example: "Operation completed successfully"},
						},
					},
				},
			},
		}
	}

	dm.addStandardErrorResponses(responses)

	return responses
}

func (dm *DocumentationManager) generateSchemas(routes map[string]*types.RouteInfo) map[string]*types.RouteSchema {
	schemas := make(map[string]*types.RouteSchema)
	schemasMu := sync.Mutex{}

	g, ctx := errgroup.WithContext(dm.ctx)

	for _, route := range routes {
		if route.Config.Doc == nil {
			continue
		}

		r := route
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				localSchemas := make(map[string]*types.RouteSchema)

				if r.Config.Doc.DocRequestType != nil {
					schemaName := dm.getTypeName(r.Config.Doc.DocRequestType)
					localSchemas[schemaName] = dm.generateSchemaFromType(r.Config.Doc.DocRequestType)
				}

				if r.Config.Doc.DocResponseType != nil {
					schemaName := dm.getTypeName(r.Config.Doc.DocResponseType)
					localSchemas[schemaName] = dm.generateSchemaFromType(r.Config.Doc.DocResponseType)
				}

				schemasMu.Lock()
				for k, v := range localSchemas {
					schemas[k] = v
				}
				schemasMu.Unlock()

				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		dm.logger.Error("Failed to generate some schemas", zap.Error(err))
	}

	schemas["ErrorResponse"] = dm.getErrorSchema()

	return schemas
}

func (dm *DocumentationManager) generateSchemaFromType(t reflect.Type) *types.RouteSchema {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		return dm.generateStructSchema(t)
	case reflect.Slice, reflect.Array:
		return &types.RouteSchema{
			Type:  "array",
			Items: dm.generateSchemaFromType(t.Elem()),
		}
	case reflect.Map:
		return &types.RouteSchema{
			Type: "object",
			Properties: map[string]*types.RouteSchema{
				"field": dm.generateSchemaFromType(t.Elem()),
			},
		}
	case reflect.String:
		return &types.RouteSchema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &types.RouteSchema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &types.RouteSchema{Type: "number"}
	case reflect.Bool:
		return &types.RouteSchema{Type: "boolean"}
	default:
		return &types.RouteSchema{Type: "object"}
	}
}

func (dm *DocumentationManager) generateStructSchema(t reflect.Type) *types.RouteSchema {
	schema := &types.RouteSchema{
		Type:       "object",
		Properties: make(map[string]*types.RouteSchema),
		Required:   make([]string, 0),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			dm.logger.Debug("Skipping non-exported field",
				zap.String("field", field.Name),
				zap.String("type", t.Name()))
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		fieldName := dm.getFieldName(field, jsonTag)
		fieldSchema := dm.generateSchemaFromField(field)

		schema.Properties[fieldName] = fieldSchema

		if dm.isFieldRequired(field) {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	return schema
}

func (dm *DocumentationManager) generateSchemaFromField(field reflect.StructField) *types.RouteSchema {
	schema := dm.generateSchemaFromType(field.Type)

	if docTag := field.Tag.Get("doc"); docTag != "" {
		schema.Description = docTag
	}

	if exampleTag := field.Tag.Get("example"); exampleTag != "" {
		switch schema.Type {
		case "string":
			schema.Example = exampleTag
		case "integer":
			if val, err := strconv.Atoi(exampleTag); err == nil {
				schema.Example = val
			} else {
				schema.Example = exampleTag
			}
		case "number":
			if val, err := strconv.ParseFloat(exampleTag, 64); err == nil {
				schema.Example = val
			} else {
				schema.Example = exampleTag
			}
		case "boolean":
			if val, err := strconv.ParseBool(exampleTag); err == nil {
				schema.Example = val
			} else {
				schema.Example = exampleTag
			}
		default:
			schema.Example = exampleTag
		}
	}

	dm.addValidationToSchema(schema, field)

	return schema
}

func (dm *DocumentationManager) extractPathParams(path string) []string {
	var params []string
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := strings.Trim(part, "{}")
			params = append(params, paramName)
		}
	}

	return params
}

func (dm *DocumentationManager) getFieldName(field reflect.StructField, jsonTag string) string {
	if jsonTag == "" {
		return strings.ToLower(field.Name)
	}

	parts := strings.Split(jsonTag, ",")
	if parts[0] != "" {
		return parts[0]
	}

	return strings.ToLower(field.Name)
}

func (dm *DocumentationManager) isFieldRequired(field reflect.StructField) bool {
	validateTag := field.Tag.Get("validate")
	return strings.Contains(validateTag, "required")
}

func (dm *DocumentationManager) getFieldDescription(field reflect.StructField) string {
	if docTag := field.Tag.Get("doc"); docTag != "" {
		return docTag
	}
	return fmt.Sprintf("%s field", field.Name)
}

func (dm *DocumentationManager) generateFieldExample(field reflect.StructField) interface{} {
	if exampleTag := field.Tag.Get("example"); exampleTag != "" {
		return exampleTag
	}

	switch field.Type.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return 1
	case reflect.Float32, reflect.Float64:
		return 1.0
	case reflect.Bool:
		return true
	default:
		return nil
	}
}

func (dm *DocumentationManager) addValidationToSchema(schema *types.RouteSchema, field reflect.StructField) {
	validateTag := field.Tag.Get("validate")
	if validateTag == "" {
		return
	}

	rules := strings.Split(validateTag, ",")
	for _, rule := range rules {
		if strings.HasPrefix(rule, "min=") {
			if val, err := strconv.ParseFloat(rule[4:], 64); err == nil {
				schema.Minimum = &val
			}
		} else if strings.HasPrefix(rule, "max=") {
			if val, err := strconv.ParseFloat(rule[4:], 64); err == nil {
				schema.Maximum = &val
			}
		} else if strings.HasPrefix(rule, "minlen=") {
			if val, err := strconv.Atoi(rule[7:]); err == nil {
				schema.MinLength = &val
			}
		} else if strings.HasPrefix(rule, "maxlen=") {
			if val, err := strconv.Atoi(rule[7:]); err == nil {
				schema.MaxLength = &val
			}
		}
	}
}

func (dm *DocumentationManager) getTypeName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func (dm *DocumentationManager) generateExampleFromSchema(schema *types.RouteSchema) interface{} {
	if schema.Example != nil {
		return schema.Example
	}

	switch schema.Type {
	case "object":
		example := make(map[string]interface{})
		for propName, propSchema := range schema.Properties {
			if propName == "$ref" {
				continue
			}

			if propSchema.Example != nil {
				example[propName] = propSchema.Example
			} else {
				example[propName] = dm.generateExampleFromSchema(propSchema)
			}
		}
		return example

	case "array":
		if schema.Items != nil {
			exampleItem := dm.generateExampleFromSchema(schema.Items)
			return []interface{}{exampleItem}
		}
		return []interface{}{}

	case "string":
		if schema.Example != nil {
			return schema.Example
		}
		return "example string"

	case "integer":
		if schema.Example != nil {
			return schema.Example
		}
		return 123

	case "number":
		if schema.Example != nil {
			return schema.Example
		}
		return 123.45

	case "boolean":
		if schema.Example != nil {
			return schema.Example
		}
		return true

	default:
		return nil
	}
}

func (dm *DocumentationManager) generateSecuritySchemes() map[string]*types.RouteSecurityScheme {
	schemes := make(map[string]*types.RouteSecurityScheme)

	schemes["ApiKeyAuth"] = &types.RouteSecurityScheme{
		Type:        "apiKey",
		In:          "header",
		Name:        "Authorization",
		Description: "API key authentication",
	}

	return schemes
}

func (dm *DocumentationManager) addStandardErrorResponses(responses map[string]*types.RouteResponse) {
	errorSchema := dm.getErrorSchema()

	responses["400"] = &types.RouteResponse{
		Description: "Bad Request",
		Content: map[string]*types.RouteMediaType{
			"application/json": {Schema: errorSchema},
		},
	}

	responses["401"] = &types.RouteResponse{
		Description: "Unauthorized",
		Content: map[string]*types.RouteMediaType{
			"application/json": {Schema: errorSchema},
		},
	}

	responses["403"] = &types.RouteResponse{
		Description: "Forbidden",
		Content: map[string]*types.RouteMediaType{
			"application/json": {Schema: errorSchema},
		},
	}

	responses["500"] = &types.RouteResponse{
		Description: "Internal Server Error",
		Content: map[string]*types.RouteMediaType{
			"application/json": {Schema: errorSchema},
		},
	}
}

func (dm *DocumentationManager) getErrorSchema() *types.RouteSchema {
	return &types.RouteSchema{
		Type: "object",
		Properties: map[string]*types.RouteSchema{
			"error": {
				Type:        "string",
				Description: "Error message",
				Example:     "Something went wrong",
			},
			"code": {
				Type:        "integer",
				Description: "Error code",
				Example:     400,
			},
		},
		Required: []string{"error", "code"},
	}
}

func (dm *DocumentationManager) inferParameterType(paramName string) string {
	typeMap := map[string]string{
		"id":      "integer",
		"count":   "integer",
		"limit":   "integer",
		"offset":  "integer",
		"page":    "integer",
		"size":    "integer",
		"active":  "boolean",
		"enabled": "boolean",
	}

	if paramType, exists := typeMap[strings.ToLower(paramName)]; exists {
		return paramType
	}

	lower := strings.ToLower(paramName)
	if strings.HasSuffix(lower, "id") || strings.HasSuffix(lower, "count") {
		return "integer"
	}
	if strings.HasSuffix(lower, "enabled") || strings.HasSuffix(lower, "active") {
		return "boolean"
	}

	return "string"
}

func (dm *DocumentationManager) generateParamDescription(paramName string) string {
	descriptions := map[string]string{
		"id":       "Unique identifier",
		"page":     "Page number for pagination",
		"limit":    "Number of items per page",
		"offset":   "Number of items to skip",
		"sort":     "Sort field and direction",
		"filter":   "Filter criteria",
		"search":   "Search query string",
		"status":   "Status filter",
		"type":     "Type filter",
		"category": "Category filter",
		"active":   "Filter by active status",
		"enabled":  "Filter by enabled status",
	}

	if desc, exists := descriptions[strings.ToLower(paramName)]; exists {
		return desc
	}

	return fmt.Sprintf("%s parameter", paramName)
}

func (dm *DocumentationManager) generateParamExample(paramName string) interface{} {
	examples := map[string]interface{}{
		"id":       123,
		"page":     1,
		"limit":    10,
		"offset":   0,
		"search":   "example",
		"status":   "active",
		"type":     "user",
		"category": "admin",
		"sort":     "name:asc",
		"filter":   "active=true",
		"active":   true,
		"enabled":  true,
	}

	if example, exists := examples[strings.ToLower(paramName)]; exists {
		return example
	}

	paramType := dm.inferParameterType(paramName)
	switch paramType {
	case "integer":
		return 1
	case "boolean":
		return true
	default:
		return "example"
	}
}

func (dm *DocumentationManager) handleDocs(ctx *types.RequestCtx) {
	if !dm.IsRunning() {
		ctx.Error(types.ErrDocsIsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

	swaggerHTML := `<!DOCTYPE html>
<html>
<head>
   <title>API Documentation</title>
   <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui.css" />
   <style>
       html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
       *, *:before, *:after { box-sizing: inherit; }
       body { margin:0; background: #fafafa; }
   </style>
</head>
<body>
   <div id="swagger-ui"></div>
   <script src="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui-bundle.js"></script>
   <script src="https://unpkg.com/swagger-ui-dist@4.15.5/swagger-ui-standalone-preset.js"></script>
   <script>
       window.onload = function() {
           const ui = SwaggerUIBundle({
               url: '/openapi.json',
               dom_id: '#swagger-ui',
               deepLinking: true,
               presets: [
                   SwaggerUIBundle.presets.apis,
                   SwaggerUIStandalonePreset
               ],
               plugins: [
                   SwaggerUIBundle.plugins.DownloadUrl
               ],
               layout: "StandaloneLayout"
           });
       };
   </script>
</body>
</html>`

	ctx.SetContentType("text/html")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_, err := ctx.Write([]byte(swaggerHTML))
	if err != nil {
		dm.logger.Error("Failed to write http writer", zap.Error(err))
	}
}

func (dm *DocumentationManager) handleOpenAPIJSON(ctx *types.RequestCtx) {
	if !dm.IsRunning() {
		ctx.Error(types.ErrDocsIsNotRunning, fasthttp.StatusServiceUnavailable)
		return
	}

	spec := dm.getSpec()

	if spec == nil {
		if err := dm.generate(); err != nil {
			dm.logger.Error("Failed to generate documentation", zap.Error(err))
			ctx.Error(err, fasthttp.StatusInternalServerError)
			return
		}
		spec = dm.getSpec()
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	specData, err := utils.Marshal(spec)
	if err != nil {
		dm.logger.Error("Failed to encode OpenAPI spec", zap.Error(err))
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	_, err = ctx.Write(specData)
	if err != nil {
		dm.logger.Error("Failed to write OpenAPI spec", zap.Error(err))
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}
}
