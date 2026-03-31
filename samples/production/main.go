package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/trofkm/compoapp"
)

// ============================================
// INFRASTRUCTURE LAYER
// ============================================

// Config holds application configuration
type Config struct {
	DatabaseURL string
	RedisURL    string
	APIKey      string
	Port        int
	LogLevel    string
}

func NewConfig() (*Config, error) {
	return &Config{
		DatabaseURL: "postgres://localhost:5432/production_db",
		RedisURL:    "redis://localhost:6379",
		APIKey:      "secret-api-key-12345",
		Port:        8080,
		LogLevel:    "info",
	}, nil
}

// Logger provides structured logging
type Logger struct {
	level string
}

func NewLogger(cfg *Config) *Logger {
	return &Logger{
		level: cfg.LogLevel,
	}
}

func (l *Logger) Info(msg string, fields ...any) {
	fmt.Printf("[INFO] "+msg+"\n", fields...)
}

func (l *Logger) Error(msg string, fields ...any) {
	fmt.Printf("[ERROR] "+msg+"\n", fields...)
}

func (l *Logger) Debug(msg string, fields ...any) {
	if l.level == "debug" {
		fmt.Printf("[DEBUG] "+msg+"\n", fields...)
	}
}

// Database represents database connection
type Database struct {
	url   string
	cache *Cache
}

func NewDatabase(cfg *Config, cache *Cache) (*Database, error) {
	// Simulate connection error possibility
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	return &Database{
		url:   cfg.DatabaseURL,
		cache: cache,
	}, nil
}

func (db *Database) Query(query string) string {
	return fmt.Sprintf("Result from DB: %s", query)
}

// Cache represents Redis cache
type Cache struct {
	url string
}

func NewCache(cfg *Config) *Cache {
	return &Cache{
		url: cfg.RedisURL,
	}
}

func (c *Cache) Get(key string) (string, bool) {
	return "cached-value", true
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
	fmt.Printf("Cache SET: %s = %s (TTL: %v)\n", key, value, ttl)
}

// HTTPClient for external API calls
type HTTPClient struct {
	client  *http.Client
	apiKey  string
	logger  *Logger
	metrics *Metrics
}

func NewHTTPClient(cfg *Config, logger *Logger, metrics *Metrics) *HTTPClient {
	return &HTTPClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		apiKey:  cfg.APIKey,
		logger:  logger,
		metrics: metrics,
	}
}

func (h *HTTPClient) Get(url string) (string, error) {
	h.metrics.Increment("http_client.requests")
	h.logger.Debug("HTTP GET: %s", url)
	return fmt.Sprintf("Response from %s", url), nil
}

// ============================================
// DOMAIN LAYER
// ============================================

// User domain model
type User struct {
	ID       string
	Email    string
	Name     string
	IsActive bool
}

// Order domain model
type Order struct {
	ID      string
	UserID  string
	Amount  float64
	Status  string
	Created time.Time
}

// ============================================
// DOMAIN INTERFACES
// ============================================

type IUserRepository interface {
	FindByID(id string) (*User, error)
	Save(user *User) error
}

type IEmailService interface {
	Send(to, subject, body string) error
}

type IPaymentGateway interface {
	ProcessPayment(orderID string, amount float64) error
}

// ============================================
// APPLICATION LAYER - REPOSITORIES
// ============================================

// UserRepository implements IUserRepository
type UserRepository struct {
	db     *Database
	cache  *Cache
	logger *Logger
}

func NewUserRepository(db *Database, cache *Cache, logger *Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		cache:  cache,
		logger: logger,
	}
}

func (r *UserRepository) FindByID(id string) (*User, error) {
	r.logger.Debug("Finding user by ID: %s", id)

	// Check cache first
	if cached, ok := r.cache.Get("user:" + id); ok {
		r.logger.Debug("Cache hit for user: %s", id)
		return &User{ID: id, Email: cached, Name: "Cached User"}, nil
	}

	// Query database
	result := r.db.Query("SELECT * FROM users WHERE id = " + id)
	r.logger.Debug("Database query result: %s", result)

	return &User{
		ID:       id,
		Email:    "user@example.com",
		Name:     "John Doe",
		IsActive: true,
	}, nil
}

func (r *UserRepository) Save(user *User) error {
	r.logger.Info("Saving user: %s", user.ID)
	r.cache.Set("user:"+user.ID, user.Email, 10*time.Minute)
	return nil
}

// OrderRepository for order persistence
type OrderRepository struct {
	db     *Database
	logger *Logger
}

func NewOrderRepository(db *Database, logger *Logger) *OrderRepository {
	return &OrderRepository{
		db:     db,
		logger: logger,
	}
}

func (r *OrderRepository) FindByID(id string) (*Order, error) {
	r.logger.Debug("Finding order by ID: %s", id)
	return &Order{
		ID:      id,
		UserID:  "user-123",
		Amount:  99.99,
		Status:  "pending",
		Created: time.Now(),
	}, nil
}

func (r *OrderRepository) Save(order *Order) error {
	r.logger.Info("Saving order: %s", order.ID)
	return nil
}

// ============================================
// APPLICATION LAYER - SERVICES
// ============================================

// EmailService implements IEmailService
type EmailService struct {
	logger  *Logger
	http    *HTTPClient
	metrics *Metrics
}

func NewEmailService(logger *Logger, http *HTTPClient, metrics *Metrics) *EmailService {
	return &EmailService{
		logger:  logger,
		http:    http,
		metrics: metrics,
	}
}

func (s *EmailService) Send(to, subject, body string) error {
	s.metrics.Increment("email.sent")
	s.logger.Info("Sending email to %s: %s", to, subject)
	return nil
}

// PaymentGateway implements IPaymentGateway
type PaymentGateway struct {
	httpClient *HTTPClient
	logger     *Logger
	metrics    *Metrics
}

func NewPaymentGateway(httpClient *HTTPClient, logger *Logger, metrics *Metrics) *PaymentGateway {
	return &PaymentGateway{
		httpClient: httpClient,
		logger:     logger,
		metrics:    metrics,
	}
}

func (g *PaymentGateway) ProcessPayment(orderID string, amount float64) error {
	g.metrics.Increment("payment.processed")
	g.logger.Info("Processing payment for order %s: $%.2f", orderID, amount)
	return nil
}

// UserService handles user business logic
type UserService struct {
	repo         IUserRepository
	emailService IEmailService
	logger       *Logger
	metrics      *Metrics
	eventBus     *EventBus
}

func NewUserService(
	repo IUserRepository,
	emailService IEmailService,
	logger *Logger,
	metrics *Metrics,
	eventBus *EventBus,
) *UserService {
	return &UserService{
		repo:         repo,
		emailService: emailService,
		logger:       logger,
		metrics:      metrics,
		eventBus:     eventBus,
	}
}

func (s *UserService) RegisterUser(email, name string) (*User, error) {
	s.metrics.Increment("user.registered")

	user := &User{
		ID:       fmt.Sprintf("user-%d", time.Now().Unix()),
		Email:    email,
		Name:     name,
		IsActive: true,
	}

	if err := s.repo.Save(user); err != nil {
		return nil, err
	}

	// Send welcome email
	s.emailService.Send(email, "Welcome!", "Welcome to our platform!")

	// Publish event
	s.eventBus.Publish("user.registered", user)

	s.logger.Info("User registered: %s", user.ID)
	return user, nil
}

func (s *UserService) GetUser(id string) (*User, error) {
	return s.repo.FindByID(id)
}

// AuthService handles authentication
type AuthService struct {
	userRepo IUserRepository
	logger   *Logger
	cache    *Cache
	jwtKey   string
}

func NewAuthService(userRepo IUserRepository, logger *Logger, cache *Cache) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		logger:   logger,
		cache:    cache,
		jwtKey:   "secret-jwt-key",
	}
}

func (s *AuthService) Authenticate(email, password string) (string, error) {
	s.logger.Info("Authenticating user: %s", email)
	// In real app, verify password hash
	return "jwt-token-" + email, nil
}

func (s *AuthService) ValidateToken(token string) (string, error) {
	s.logger.Debug("Validating token: %s", token)
	// In real app, parse and validate JWT
	return "user-123", nil
}

// OrderService handles order business logic
type OrderService struct {
	orderRepo    *OrderRepository
	userRepo     IUserRepository
	payment      IPaymentGateway
	emailService IEmailService
	logger       *Logger
	metrics      *Metrics
	eventBus     *EventBus
}

func NewOrderService(
	orderRepo *OrderRepository,
	userRepo IUserRepository,
	payment IPaymentGateway,
	emailService IEmailService,
	logger *Logger,
	metrics *Metrics,
	eventBus *EventBus,
) *OrderService {
	return &OrderService{
		orderRepo:    orderRepo,
		userRepo:     userRepo,
		payment:      payment,
		emailService: emailService,
		logger:       logger,
		metrics:      metrics,
		eventBus:     eventBus,
	}
}

func (s *OrderService) CreateOrder(userID string, amount float64) (*Order, error) {
	s.metrics.Increment("order.created")

	// Verify user exists
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	order := &Order{
		ID:      fmt.Sprintf("order-%d", time.Now().Unix()),
		UserID:  userID,
		Amount:  amount,
		Status:  "pending",
		Created: time.Now(),
	}

	// Process payment
	if err := s.payment.ProcessPayment(order.ID, amount); err != nil {
		s.logger.Error("Payment failed for order %s: %v", order.ID, err)
		return nil, err
	}

	order.Status = "paid"
	if err := s.orderRepo.Save(order); err != nil {
		return nil, err
	}

	// Send confirmation email
	s.emailService.Send(user.Email, "Order Confirmed",
		fmt.Sprintf("Your order %s has been confirmed", order.ID))

	// Publish event
	s.eventBus.Publish("order.created", order)

	s.logger.Info("Order created: %s", order.ID)
	return order, nil
}

// ============================================
// CROSS-CUTTING CONCERNS
// ============================================

// Metrics collector
type Metrics struct {
	counters map[string]int
	logger   *Logger
}

func NewMetrics(logger *Logger) *Metrics {
	return &Metrics{
		counters: make(map[string]int),
		logger:   logger,
	}
}

func (m *Metrics) Increment(name string) {
	m.counters[name]++
	m.logger.Debug("Metric incremented: %s = %d", name, m.counters[name])
}

func (m *Metrics) Get(name string) int {
	return m.counters[name]
}

// EventBus for domain events
type EventBus struct {
	subscribers map[string][]func(any)
	logger      *Logger
}

func NewEventBus(logger *Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[string][]func(any)),
		logger:      logger,
	}
}

func (b *EventBus) Subscribe(event string, handler func(any)) {
	b.subscribers[event] = append(b.subscribers[event], handler)
	b.logger.Debug("Subscribed to event: %s", event)
}

func (b *EventBus) Publish(event string, data any) {
	b.logger.Info("Publishing event: %s", event)
	if handlers, ok := b.subscribers[event]; ok {
		for _, handler := range handlers {
			handler(data)
		}
	}
}

// ============================================
// BACKGROUND WORKERS
// ============================================

// EmailWorker processes email queue
type EmailWorker struct {
	emailService IEmailService
	logger       *Logger
	eventBus     *EventBus
}

func NewEmailWorker(emailService IEmailService, logger *Logger, eventBus *EventBus) *EmailWorker {
	worker := &EmailWorker{
		emailService: emailService,
		logger:       logger,
		eventBus:     eventBus,
	}

	// Subscribe to events
	eventBus.Subscribe("user.registered", worker.handleUserRegistered)
	eventBus.Subscribe("order.created", worker.handleOrderCreated)

	return worker
}

func (w *EmailWorker) handleUserRegistered(data any) {
	user, ok := data.(*User)
	if !ok {
		w.logger.Error("Invalid user data in event")
		return
	}
	w.logger.Info("EmailWorker: Processing user registration for %s", user.Email)
}

func (w *EmailWorker) handleOrderCreated(data any) {
	order, ok := data.(*Order)
	if !ok {
		w.logger.Error("Invalid order data in event")
		return
	}
	w.logger.Info("EmailWorker: Processing order notification for %s", order.ID)
}

// CacheWarmer pre-warms cache
type CacheWarmer struct {
	cache  *Cache
	logger *Logger
}

func NewCacheWarmer(cache *Cache, logger *Logger) *CacheWarmer {
	return &CacheWarmer{
		cache:  cache,
		logger: logger,
	}
}

func (w *CacheWarmer) WarmUp() {
	w.logger.Info("Warming up cache...")
	w.cache.Set("warmup:key1", "value1", 5*time.Minute)
	w.cache.Set("warmup:key2", "value2", 5*time.Minute)
}

// ============================================
// PRESENTATION LAYER
// ============================================

// UserHandler handles HTTP requests for users
type UserHandler struct {
	userService *UserService
	authService *AuthService
	logger      *Logger
}

func NewUserHandler(userService *UserService, authService *AuthService, logger *Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		authService: authService,
		logger:      logger,
	}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Handling user registration")
	user, err := h.userService.RegisterUser("new@example.com", "New User")
	if err != nil {
		h.logger.Error("Registration failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "User registered: %s\n", user.ID)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Handling user login")
	token, err := h.authService.Authenticate("user@example.com", "password")
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(w, "Token: %s\n", token)
}

// OrderHandler handles HTTP requests for orders
type OrderHandler struct {
	orderService *OrderService
	authService  *AuthService
	logger       *Logger
}

func NewOrderHandler(orderService *OrderService, authService *AuthService, logger *Logger) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		authService:  authService,
		logger:       logger,
	}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Handling order creation")
	order, err := h.orderService.CreateOrder("user-123", 149.99)
	if err != nil {
		h.logger.Error("Order creation failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Order created: %s, Amount: $%.2f\n", order.ID, order.Amount)
}

// AuthMiddleware validates JWT tokens
type AuthMiddleware struct {
	authService *AuthService
	logger      *Logger
}

func NewAuthMiddleware(authService *AuthService, logger *Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		logger:      logger,
	}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := m.authService.ValidateToken(token)
		if err != nil {
			m.logger.Error("Token validation failed: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		m.logger.Debug("Authenticated user: %s", userID)
		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Server represents HTTP server
type Server struct {
	userHandler    *UserHandler
	orderHandler   *OrderHandler
	authMiddleware *AuthMiddleware
	logger         *Logger
	config         *Config
	httpServer     *http.Server
}

func NewServer(
	userHandler *UserHandler,
	orderHandler *OrderHandler,
	authMiddleware *AuthMiddleware,
	logger *Logger,
	config *Config,
) *Server {
	return &Server{
		userHandler:    userHandler,
		orderHandler:   orderHandler,
		authMiddleware: authMiddleware,
		logger:         logger,
		config:         config,
	}
}

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/register", s.userHandler.Register)
	mux.HandleFunc("/login", s.userHandler.Login)

	// Protected routes
	protected := http.NewServeMux()
	protected.HandleFunc("/orders", s.orderHandler.CreateOrder)

	// Apply middleware
	mux.Handle("/orders", s.authMiddleware.RequireAuth(protected))

	return mux
}

func (s *Server) Start() error {
	mux := s.SetupRoutes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.logger.Info("Server starting on port %d", s.config.Port)

	// In production, handle this properly with graceful shutdown
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server error: %v", err)
		}
	}()

	return nil
}

// ============================================
// APPLICATION COMPOSITION ROOT
// ============================================

type Application struct {
	server      *Server
	cacheWarmer *CacheWarmer
	emailWorker *EmailWorker
	logger      *Logger
	metrics     *Metrics
}

func NewApplication(
	server *Server,
	cacheWarmer *CacheWarmer,
	emailWorker *EmailWorker,
	logger *Logger,
	metrics *Metrics,
) *Application {
	return &Application{
		server:      server,
		cacheWarmer: cacheWarmer,
		emailWorker: emailWorker,
		logger:      logger,
		metrics:     metrics,
	}
}

func (app *Application) Run() error {
	app.logger.Info("Starting application...")

	// Warm up cache
	app.cacheWarmer.WarmUp()

	// Start server
	if err := app.server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	app.logger.Info("Application started successfully")

	// Simulate some requests
	app.logger.Info("\n=== Simulating Production Requests ===\n")

	// 1. User Registration
	app.logger.Info("1. User Registration:")
	regReq := httptest.NewRequest("POST", "/register", bytes.NewBufferString(`{"email":"new@example.com","name":"New User"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	app.server.userHandler.Register(regW, regReq)
	app.logger.Info("Response: %s", regW.Body.String())

	// 2. User Login
	app.logger.Info("\n2. User Login:")
	loginReq := httptest.NewRequest("POST", "/login", bytes.NewBufferString(`{"email":"user@example.com","password":"password"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	app.server.userHandler.Login(loginW, loginReq)
	app.logger.Info("Response: %s", loginW.Body.String())

	// 3. Create Order (with auth token)
	app.logger.Info("\n3. Create Order:")
	orderReq := httptest.NewRequest("POST", "/orders", bytes.NewBufferString(`{"user_id":"user-123","amount":149.99}`))
	orderReq.Header.Set("Content-Type", "application/json")
	orderReq.Header.Set("Authorization", "Bearer jwt-token-user@example.com")
	orderW := httptest.NewRecorder()
	app.server.orderHandler.CreateOrder(orderW, orderReq)
	app.logger.Info("Response: %s", orderW.Body.String())

	// Show metrics
	app.logger.Info("\n=== Metrics ===")
	app.logger.Info("Users registered: %d", app.metrics.Get("user.registered"))
	app.logger.Info("Orders created: %d", app.metrics.Get("order.created"))
	app.logger.Info("Emails sent: %d", app.metrics.Get("email.sent"))
	app.logger.Info("Payments processed: %d", app.metrics.Get("payment.processed"))
	app.logger.Info("HTTP requests: %d", app.metrics.Get("http_client.requests"))

	return nil
}

// ============================================
// MAIN - COMPOSITION ROOT
// ============================================

func main() {
	fmt.Println("=== Production DI Container Example ===\n")
	fmt.Println("This example demonstrates:")
	fmt.Println("  ✓ Multi-layer architecture (Infrastructure, Domain, Application, Presentation)")
	fmt.Println("  ✓ Interface resolution (IUserRepository, IEmailService, IPaymentGateway)")
	fmt.Println("  ✓ Complex dependency graph with 20+ components")
	fmt.Println("  ✓ Cross-cutting concerns (Metrics, EventBus, Logging)")
	fmt.Println("  ✓ Background workers with event-driven communication")
	fmt.Println("  ✓ Constructor error handling")
	fmt.Println("  ✓ Real-world production patterns")
	fmt.Println("\nInitializing DI Container...\n")

	// Create container with debug mode
	container := compoapp.NewContainer()
	container.Debug()

	// ============================================
	// INFRASTRUCTURE LAYER
	// ============================================
	container.MustProvide(NewConfig)
	container.MustProvide(NewLogger)
	container.MustProvide(NewMetrics)
	container.MustProvide(NewCache)
	container.MustProvide(NewDatabase) // Returns (*Database, error)
	container.MustProvide(NewHTTPClient)

	// ============================================
	// CROSS-CUTTING CONCERNS
	// ============================================
	container.MustProvide(NewEventBus)

	// ============================================
	// REPOSITORIES (implement interfaces)
	// ============================================
	container.MustProvide(NewUserRepository) // Implements IUserRepository
	container.MustProvide(NewOrderRepository)

	// ============================================
	// EXTERNAL SERVICES (implement interfaces)
	// ============================================
	container.MustProvide(NewEmailService)   // Implements IEmailService
	container.MustProvide(NewPaymentGateway) // Implements IPaymentGateway

	// ============================================
	// APPLICATION SERVICES
	// ============================================
	container.MustProvide(NewUserService)
	container.MustProvide(NewAuthService)
	container.MustProvide(NewOrderService)

	// ============================================
	// BACKGROUND WORKERS
	// ============================================
	container.MustProvide(NewEmailWorker)
	container.MustProvide(NewCacheWarmer)

	// ============================================
	// PRESENTATION LAYER
	// ============================================
	container.MustProvide(NewUserHandler)
	container.MustProvide(NewOrderHandler)
	container.MustProvide(NewAuthMiddleware)
	container.MustProvide(NewServer)

	// ============================================
	// APPLICATION ROOT
	// ============================================
	container.MustProvide(NewApplication)

	// Resolve the entire application
	fmt.Println("Resolving dependencies...\n")
	var app *Application
	// todo: visualize work only after must resolve
	if err := container.Resolve(&app); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Visualize dependency graph
	if err := container.Visualize("dependency_graph.dot"); err != nil {
		fmt.Printf("Failed to visualize graph: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Println("Dependency graph saved to: dependency_graph.dot")
		fmt.Println("To visualize: dot -Tpng dependency_graph.dot -o graph.png\n")
	}

	// Run the application
	fmt.Println("\n=== Running Application ===\n")
	if err := app.Run(); err != nil {
		panic(err)
	}

	fmt.Println("\n✓ Application completed successfully!")
}
