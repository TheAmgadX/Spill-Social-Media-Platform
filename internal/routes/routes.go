package routes

import (
	"net/http"
	"strings"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/controllers"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/middleware"
	"github.com/gorilla/mux"
)

type RouterConfig struct {
	AuthController        *controllers.AuthController
	UserController        *controllers.UserController
	PostController        *controllers.PostController
	InteractionController *controllers.InteractionController
	CommentController     *controllers.CommentController
	AssetController       *controllers.AssetController
	SearchController      *controllers.SearchController
	JWTSecret             []byte
}

func SetupRouter(cfg RouterConfig) *mux.Router {
	r := mux.NewRouter()

	// Max request body size 1MB for all JSON payloads
	r.Use(middleware.MaxBytes(1 << 20))

	r.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		probeMethods := []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		}
		var allowed []string
		for _, m := range probeMethods {
			probe := req.Clone(req.Context())
			probe.Method = m
			var match mux.RouteMatch
			if r.Match(probe, &match) && match.MatchErr == nil {
				allowed = append(allowed, m)
			}
		}
		if len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	requireAuth := middleware.RequireAuth(cfg.JWTSecret)
	optionalAuth := middleware.OptionalAuth(cfg.JWTSecret)

	registerRoutes := func(prefix string) {
		// Auth endpoints
		if cfg.AuthController != nil {
			r.HandleFunc(prefix+"/auth/request-code", cfg.AuthController.RequestCode).Methods(http.MethodPost)
			r.HandleFunc(prefix+"/auth/verify", cfg.AuthController.Verify).Methods(http.MethodPost)
			r.HandleFunc(prefix+"/auth/signup", cfg.AuthController.Signup).Methods(http.MethodPost)
			r.HandleFunc(prefix+"/auth/refresh", cfg.AuthController.Refresh).Methods(http.MethodPost)
			r.HandleFunc(prefix+"/auth/logout", cfg.AuthController.Logout).Methods(http.MethodPost)
		}

		// Search (with OptionalAuth)
		if cfg.SearchController != nil {
			r.Handle(prefix+"/search", optionalAuth(http.HandlerFunc(cfg.SearchController.Search))).Methods(http.MethodGet)
		}

		// Public users endpoints
		if cfg.UserController != nil {
			r.HandleFunc(prefix+"/users/{id}/followers", cfg.UserController.GetFollowers).Methods(http.MethodGet)
			r.HandleFunc(prefix+"/users/{id}/following", cfg.UserController.GetFollowing).Methods(http.MethodGet)
			r.HandleFunc(prefix+"/users/{username}", cfg.UserController.GetProfile).Methods(http.MethodGet)
		}

		// Posts read endpoints (with OptionalAuth)
		if cfg.PostController != nil {
			r.Handle(prefix+"/users/{id}/posts", optionalAuth(http.HandlerFunc(cfg.PostController.GetUserPosts))).Methods(http.MethodGet)
			r.Handle(prefix+"/posts/{id}", optionalAuth(http.HandlerFunc(cfg.PostController.GetPost))).Methods(http.MethodGet)
		}

		// Comments read endpoint
		if cfg.CommentController != nil {
			r.HandleFunc(prefix+"/posts/{id}/comments", cfg.CommentController.GetComments).Methods(http.MethodGet)
		}

		// Protected endpoints (RequireAuth)
		handleProtected := func(path string, handler http.HandlerFunc, methods ...string) {
			r.Handle(prefix+path, requireAuth(handler)).Methods(methods...)
		}

		// Me
		if cfg.UserController != nil {
			handleProtected("/me", cfg.UserController.GetMe, http.MethodGet)
			handleProtected("/me", cfg.UserController.UpdateMe, http.MethodPatch)
			handleProtected("/users/{id}/follow", cfg.UserController.Follow, http.MethodPut)
			handleProtected("/users/{id}/follow", cfg.UserController.Unfollow, http.MethodDelete)
		}

		// Posts & Feed
		if cfg.PostController != nil {
			handleProtected("/feed", cfg.PostController.GetFeed, http.MethodGet)
			handleProtected("/posts", cfg.PostController.CreatePost, http.MethodPost)
			handleProtected("/posts/{id}", cfg.PostController.UpdatePost, http.MethodPatch)
			handleProtected("/posts/{id}", cfg.PostController.DeletePost, http.MethodDelete)
		}

		// Interactions
		if cfg.InteractionController != nil {
			handleProtected("/posts/{id}/like", cfg.InteractionController.Like, http.MethodPut)
			handleProtected("/posts/{id}/like", cfg.InteractionController.Unlike, http.MethodDelete)
			handleProtected("/posts/{id}/share", cfg.InteractionController.Share, http.MethodPost)
		}

		// Comments
		if cfg.CommentController != nil {
			handleProtected("/posts/{id}/comments", cfg.CommentController.CreateComment, http.MethodPost)
			handleProtected("/comments/{id}", cfg.CommentController.DeleteComment, http.MethodDelete)
		}

		// Assets
		if cfg.AssetController != nil {
			handleProtected("/assets", cfg.AssetController.CreateAsset, http.MethodPost)
			handleProtected("/assets/{id}/complete", cfg.AssetController.CompleteAsset, http.MethodPost)
		}
	}

	// Register /api/v1 routes and root routes directly on r (avoids gorilla/mux subrouter ErrMethodMismatch masking)
	registerRoutes("/api/v1")
	registerRoutes("")

	return r
}
