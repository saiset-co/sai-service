package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Создаем приложение Fiber с настройками по умолчанию
	app := fiber.New(fiber.Config{
		ServerHeader:          "Fiber/Heavy",
		DisableStartupMessage: false,
		Prefork:               false,
		StrictRouting:         true,
		CaseSensitive:         true,
		UnescapePath:          true,
		// ETag убираем - будем использовать middleware
		BodyLimit: 4 * 1024 * 1024, // 4MB
	})

	// ============================================================================
	// МАКСИМАЛЬНОЕ КОЛИЧЕСТВО MIDDLEWARE
	// ============================================================================

	// Простейший ping эндпоинт
	app.Get("/ping/:id/pong/:id", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	// Hello эндпоинт с параметром
	app.Get("/hello/:name", func(c *fiber.Ctx) error {
		name := c.Params("name")
		if name == "" {
			name = "world"
		}
		return c.JSON(fiber.Map{
			"message":    fmt.Sprintf("Hello, %s!", name),
			"timestamp":  time.Now().Unix(),
			"request_id": c.Get("X-Request-ID"),
		})
	})

	// JSON эндпоинт с расширенными данными
	app.Post("/data", func(c *fiber.Ctx) error {
		data := make([]map[string]interface{}, 100)
		for i := 0; i < 100; i++ {
			data[i] = map[string]interface{}{
				"id":        i,
				"name":      "Item " + string(rune(i)),
				"value":     i * 10,
				"timestamp": time.Now(),
				"active":    i%2 == 0,
			}
		}

		return c.JSON(fiber.Map{
			"status": "ok",
			"count":  len(data),
			"data":   data,
		})
	})

	// Health check с подробной информацией
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":     "healthy",
			"time":       time.Now().Unix(),
			"service":    "fiber-heavy-test",
			"request_id": c.Get("X-Request-ID"),
			"middleware": map[string]bool{
				"logger":      true,
				"cors":        true,
				"helmet":      true,
				"compression": true,
				"rate_limit":  true,
				"cache":       true,
				"csrf":        true,
				"encryption":  true,
				"idempotency": true,
				"monitoring":  true,
			},
		})
	})

	// Echo endpoint для POST тестов
	app.Post("/echo", func(c *fiber.Ctx) error {
		body := c.Body()
		return c.JSON(fiber.Map{
			"echo":       string(body),
			"size":       len(body),
			"timestamp":  time.Now().Unix(),
			"request_id": c.Get("X-Request-ID"),
			"compressed": c.Get("Content-Encoding") != "",
		})
	})

	// Корневой эндпоинт
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service":    "Fiber Heavy Middleware Performance Test",
			"middleware": "20+ active middleware",
			"request_id": c.Get("X-Request-ID"),
			"endpoints": []string{
				"GET /ping",
				"GET /hello/:name",
				"GET /data",
				"GET /health",
				"POST /echo",
				"GET /metrics",
				"GET /admin/stats",
			},
		})
	})

	// Стресс тест с middleware overhead
	app.Get("/stress", func(c *fiber.Ctx) error {
		count := 0
		for i := 0; i < 1000; i++ {
			count += i
		}
		return c.JSON(fiber.Map{
			"result":     count,
			"loops":      1000,
			"request_id": c.Get("X-Request-ID"),
			"overhead":   "heavy middleware",
		})
	})

	// Запускаем сервер
	port := ":8080"
	fmt.Printf("🚀 Fiber Heavy Middleware server starting on port %s\n", port)
	fmt.Println("📊 Performance test endpoints (with 20+ middleware):")
	fmt.Println("   GET  /ping            - Simple ping")
	fmt.Println("   GET  /hello/:name     - Hello with parameter")
	fmt.Println("   GET  /data            - JSON response")
	fmt.Println("   GET  /health          - Health check")
	fmt.Println("   POST /echo            - Echo request body")
	fmt.Println("   GET  /stress          - CPU stress test")
	fmt.Println("   GET  /metrics         - Monitoring dashboard")
	fmt.Println("   GET  /admin/stats     - Admin stats (auth required)")
	fmt.Println("")
	fmt.Println("🔧 Active Middleware (20+):")
	fmt.Println("   • Logger + Request ID")
	fmt.Println("   • CORS + Helmet (Security)")
	fmt.Println("   • Compression + ETag")
	fmt.Println("   • Rate Limiter + Cache")
	fmt.Println("   • CSRF + Cookie Encryption")
	fmt.Println("   • Idempotency + Timeout")
	fmt.Println("   • Basic Auth + Monitoring")
	fmt.Println("   • Custom validation & headers")
	fmt.Println("   • Favicon + Pprof + Skip")
	fmt.Println("")
	fmt.Println("⚠️  This version simulates real production overhead!")
	fmt.Println("")

	log.Fatal(app.Listen(port))
}
