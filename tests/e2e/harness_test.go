//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/config"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/controllers"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/db"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/middleware"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/models"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/routes"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var otpCodeRegex = regexp.MustCompile(`\b([0-9]{6})\b`)

// stubMailer captures OTP emails in memory so tests run deterministically offline.
type stubMailer struct {
	mu       sync.Mutex
	messages map[string][]string // recipient email -> list of message bodies
}

func newStubMailer() *stubMailer {
	return &stubMailer{
		messages: make(map[string][]string),
	}
}

func (m *stubMailer) Send(to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(to))
	m.messages[key] = append(m.messages[key], body)
	return nil
}

func (m *stubMailer) latestOTP(t *testing.T, email string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(email))
	msgs := m.messages[key]
	if len(msgs) == 0 {
		t.Fatalf("no OTP email captured for %s", email)
	}
	latest := msgs[len(msgs)-1]
	matches := otpCodeRegex.FindStringSubmatch(latest)
	if len(matches) < 2 {
		t.Fatalf("failed to extract 6-digit OTP from email body for %s: %q", email, latest)
	}
	return matches[1]
}

// stubStorageServer provides an in-memory HTTP server mimicking Supabase Storage
// APIs so storage.Client runs its real HTTP code deterministically offline.
type stubStorageServer struct {
	server  *httptest.Server
	mu      sync.Mutex
	objects map[string]int64 // "<bucket>/<objectKey>" -> uploaded byte size
}

func newStubStorageServer() *stubStorageServer {
	s := &stubStorageServer{
		objects: make(map[string]int64),
	}
	mux := http.NewServeMux()

	// POST /storage/v1/object/upload/sign/{bucket}/{key...}
	mux.HandleFunc("/storage/v1/object/upload/sign/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/storage/v1/object/upload/sign/")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"url":   fmt.Sprintf("%s/upload/signed/%s?token=e2e-stub-token", s.server.URL, rest),
			"token": "e2e-stub-token",
		})
	})

	// PUT /upload/signed/{bucket}/{key...} (presigned PUT upload target)
	mux.HandleFunc("/upload/signed/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/upload/signed/")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.objects[key] = int64(len(body))
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	// GET /storage/v1/object/info/{bucket}/{key...}
	mux.HandleFunc("/storage/v1/object/info/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/storage/v1/object/info/")
		s.mu.Lock()
		size, ok := s.objects[key]
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{
			"size": size,
		})
	})

	// DELETE /storage/v1/object/{bucket}
	mux.HandleFunc("/storage/v1/object/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		bucket := strings.TrimPrefix(r.URL.Path, "/storage/v1/object/")
		var payload struct {
			Prefixes []string `json:"prefixes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		s.mu.Lock()
		for _, p := range payload.Prefixes {
			delete(s.objects, bucket+"/"+p)
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	s.server = httptest.NewServer(mux)
	return s
}

func (s *stubStorageServer) close() {
	s.server.Close()
}

type e2eEnv struct {
	server        *httptest.Server
	storageServer *stubStorageServer
	mailer        *stubMailer
	pool          *pgxpool.Pool
	cfg           *config.Config

	mu             sync.Mutex
	createdUserIDs []string
	createdEmails  []string
}

type testSession struct {
	UserID       string
	Username     string
	Email        string
	DisplayName  string
	AccessToken  string
	RefreshToken string
	Client       *http.Client
}

func loadDotEnvFromRepoRoot() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 6; i++ {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

func loadE2EConfig(t *testing.T) *config.Config {
	t.Helper()
	loadDotEnvFromRepoRoot()

	dbURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dbURL == "" {
		dbURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dbURL == "" {
		t.Fatal("TEST_DATABASE_URL or DATABASE_URL must be set (via environment or .env)")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		jwtSecret = "e2e-test-jwt-secret-key-at-least-32-bytes"
	}
	otpHMACKey := os.Getenv("OTP_HMAC_KEY")
	if len(otpHMACKey) < 32 {
		otpHMACKey = "e2e-test-otp-hmac-key-at-least-32-bytes"
	}
	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "social-api"
	}
	storageBucket := os.Getenv("STORAGE_BUCKET")
	if storageBucket == "" {
		storageBucket = "media"
	}

	return &config.Config{
		AppEnv:            "test",
		HTTPPort:          "0",
		DatabaseURL:       dbURL,
		StorageBucket:     storageBucket,
		JWTSecret:         []byte(jwtSecret),
		JWTIssuer:         jwtIssuer,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   720 * time.Hour,
		SignupTokenTTL:    10 * time.Minute,
		OTPHMACKey:        []byte(otpHMACKey),
		OTPTTL:            10 * time.Minute,
		OTPResendCooldown: 60 * time.Second,
		OTPMaxAttempts:    5,
		Mailer:            "stub",
	}
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()

	cfg := loadE2EConfig(t)
	ctx := context.Background()

	appPool, err := db.NewAppPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("failed to connect to PostgreSQL: %v", err)
	}

	stubStore := newStubStorageServer()
	cfg.SupabaseURL = stubStore.server.URL
	cfg.SupabaseServiceRoleKey = "e2e-service-role-key"
	cfg.StoragePublicBaseURL = fmt.Sprintf("%s/storage/v1/object/public/%s", stubStore.server.URL, cfg.StorageBucket)

	stubMail := newStubMailer()
	storageClient := storage.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey)
	repo := models.NewRepository(appPool.Pool)

	authService := services.NewAuthService(repo, stubMail, cfg)
	userService := services.NewUserService(repo, cfg.StoragePublicBaseURL)
	postService := services.NewPostService(repo, cfg.StoragePublicBaseURL)
	commentService := services.NewCommentService(repo, cfg.StoragePublicBaseURL)
	assetService := services.NewAssetService(repo, storageClient, cfg.StorageBucket, cfg.StoragePublicBaseURL)
	searchService := services.NewSearchService(repo, cfg.StoragePublicBaseURL)

	r := routes.SetupRouter(routes.RouterConfig{
		AuthController:        controllers.NewAuthController(authService, cfg.JWTSecret),
		UserController:        controllers.NewUserController(userService),
		PostController:        controllers.NewPostController(postService),
		InteractionController: controllers.NewInteractionController(postService),
		CommentController:     controllers.NewCommentController(commentService),
		AssetController:       controllers.NewAssetController(assetService),
		SearchController:      controllers.NewSearchController(searchService),
		JWTSecret:             cfg.JWTSecret,
	})

	// Mount the same middleware stack as cmd/main.go
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	server := httptest.NewServer(r)

	env := &e2eEnv{
		server:        server,
		storageServer: stubStore,
		mailer:        stubMail,
		pool:          appPool.Pool,
		cfg:           cfg,
	}

	t.Cleanup(func() {
		server.Close()
		stubStore.close()
		env.cleanupDB()
		appPool.Close()
	})

	return env
}

func (e *e2eEnv) trackUser(userID, email string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if userID != "" {
		e.createdUserIDs = append(e.createdUserIDs, userID)
	}
	if email != "" {
		e.createdEmails = append(e.createdEmails, strings.ToLower(strings.TrimSpace(email)))
	}
}

func (e *e2eEnv) trackEmail(email string) {
	e.trackUser("", email)
}

func (e *e2eEnv) cleanupDB() {
	e.mu.Lock()
	userIDs := append([]string(nil), e.createdUserIDs...)
	emails := append([]string(nil), e.createdEmails...)
	e.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if len(userIDs) > 0 {
		// post_assets.asset_id does not cascade on asset delete, so remove post_assets first.
		_, _ = e.pool.Exec(ctx, `
DELETE FROM public.post_assets
WHERE post_id IN (SELECT id FROM public.posts WHERE author_id = ANY($1))
   OR asset_id IN (SELECT id FROM public.assets WHERE owner_id = ANY($1));`, userIDs)

		_, _ = e.pool.Exec(ctx, `UPDATE public.users SET avatar_asset_id = NULL WHERE id = ANY($1);`, userIDs)
		_, _ = e.pool.Exec(ctx, `DELETE FROM public.posts WHERE author_id = ANY($1);`, userIDs)
		_, _ = e.pool.Exec(ctx, `DELETE FROM public.assets WHERE owner_id = ANY($1);`, userIDs)
		_, _ = e.pool.Exec(ctx, `DELETE FROM public.users WHERE id = ANY($1);`, userIDs)
	}

	if len(emails) > 0 {
		_, _ = e.pool.Exec(ctx, `DELETE FROM public.login_codes WHERE email = ANY($1);`, emails)
	}
}

func newUniqueSuffix(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("failed to generate random suffix: %v", err)
	}
	return hex.EncodeToString(b[:])
}

func newHTTPClient(t *testing.T, withCookieJar bool) *http.Client {
	t.Helper()
	var jar http.CookieJar
	if withCookieJar {
		var err error
		jar, err = cookiejar.New(nil)
		if err != nil {
			t.Fatalf("failed to create cookie jar: %v", err)
		}
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
	}
}

func (e *e2eEnv) doJSON(t *testing.T, client *http.Client, method, path, bearerToken string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body for %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatalf("failed to build request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP %s %s failed: %v", method, path, err)
	}
	return resp
}

func readBodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return string(b)
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("failed to decode JSON (%s): %v", string(b), err)
	}
	return out
}

func requireStatus(t *testing.T, resp *http.Response, expected int, action string) {
	t.Helper()
	if resp.StatusCode != expected {
		body := readBodyString(t, resp)
		t.Fatalf("%s: expected HTTP %d, got %d; body: %s", action, expected, resp.StatusCode, body)
	}
}

// registerUser executes the real HTTP onboarding flow (request-code -> verify -> signup)
// and tracks the created account for cleanup.
func (e *e2eEnv) registerUser(t *testing.T, tag string) *testSession {
	t.Helper()
	suffix := newUniqueSuffix(t)
	email := fmt.Sprintf("e2e_%s_%s@spill-test.example", tag, suffix)
	username := fmt.Sprintf("e2e_%s_%s", tag, suffix)
	if len(username) > 20 {
		username = username[:20]
	}
	username = strings.TrimRight(username, "_")
	displayName := fmt.Sprintf("E2E %s %s", tag, suffix)

	e.trackEmail(email)
	client := newHTTPClient(t, true)

	// 1. POST /auth/request-code
	resp := e.doJSON(t, client, http.MethodPost, "/auth/request-code", "", map[string]string{
		"email": email,
	})
	requireStatus(t, resp, http.StatusAccepted, "request-code for "+username)
	_ = readBodyString(t, resp)

	// 2. POST /auth/verify
	otp := e.mailer.latestOTP(t, email)
	resp = e.doJSON(t, client, http.MethodPost, "/auth/verify", "", map[string]string{
		"email": email,
		"code":  otp,
	})
	requireStatus(t, resp, http.StatusOK, "verify for "+username)
	verifyRes := decodeJSON[struct {
		Action      string `json:"action"`
		NewUser     bool   `json:"new_user"`
		SignupToken string `json:"signupToken"`
	}](t, resp)
	if verifyRes.Action != "signup" || !verifyRes.NewUser || verifyRes.SignupToken == "" {
		t.Fatalf("unexpected verify response for new user %s: %+v", email, verifyRes)
	}

	// 3. POST /auth/signup
	resp = e.doJSON(t, client, http.MethodPost, "/auth/signup", "", map[string]string{
		"signupToken": verifyRes.SignupToken,
		"username":    username,
		"displayName": displayName,
		"birthdate":   "1997-04-15",
	})
	requireStatus(t, resp, http.StatusCreated, "signup for "+username)
	signupRes := decodeJSON[struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		User         struct {
			ID          string `json:"id"`
			Username    string `json:"username"`
			Email       string `json:"email"`
			DisplayName string `json:"displayName"`
		} `json:"user"`
	}](t, resp)

	e.trackUser(signupRes.User.ID, email)

	return &testSession{
		UserID:       signupRes.User.ID,
		Username:     signupRes.User.Username,
		Email:        signupRes.User.Email,
		DisplayName:  signupRes.User.DisplayName,
		AccessToken:  signupRes.AccessToken,
		RefreshToken: signupRes.RefreshToken,
		Client:       client,
	}
}
