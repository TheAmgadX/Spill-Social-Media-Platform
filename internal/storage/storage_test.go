package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/storage"
)

func TestStorage_CreateSignedUploadURL(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		expectedURL string
		expectErr   bool
		errContains string
	}{
		{
			name: "Success with absolute URL",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("apikey") != "test-key" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"url": "https://storage.supabase.co/media/upload?token=123",
				})
			},
			expectedURL: "https://storage.supabase.co/media/upload?token=123",
			expectErr:   false,
		},
		{
			name: "Success with /storage/v1 prefix",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"url": "/storage/v1/object/upload/media/item",
				})
			},
			expectedURL: "/storage/v1/object/upload/media/item", // will be prefixed by supabaseURL
			expectErr:   false,
		},
		{
			name: "Success with / prefix",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"url": "/object/upload/media/item",
				})
			},
			expectedURL: "/storage/v1/object/upload/media/item", // will be prefixed by supabaseURL
			expectErr:   false,
		},
		{
			name: "Success with relative path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"url": "object/upload/media/item",
				})
			},
			expectedURL: "/storage/v1/object/upload/media/item", // will be prefixed by supabaseURL
			expectErr:   false,
		},
		{
			name: "Supabase returns 401 unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("invalid api key"))
			},
			expectErr:   true,
			errContains: "supabase sign upload returned 401",
		},
		{
			name: "Supabase returns 500 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal database error"))
			},
			expectErr:   true,
			errContains: "supabase sign upload returned 500",
		},
		{
			name: "Supabase returns invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("not valid json"))
			},
			expectErr:   true,
			errContains: "failed to decode upload sign response",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			client := storage.NewClient(server.URL, "test-key")
			resURL, err := client.CreateSignedUploadURL(context.Background(), "media", "u/123/file.jpg")

			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !errors.Is(err, errors.New(tc.errContains)) && !containsString(err.Error(), tc.errContains) {
					t.Errorf("expected error containing %q, got %v", tc.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tc.expectedURL != "" && !containsString(resURL, tc.expectedURL) {
					t.Errorf("expected URL containing %q, got %q", tc.expectedURL, resURL)
				}
			}
		})
	}
}

func TestStorage_VerifyObject(t *testing.T) {
	tests := []struct {
		name         string
		expectedSize int64
		handler      http.HandlerFunc
		expectedErr  error
	}{
		{
			name:         "Success size matches",
			expectedSize: 1024,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]int64{"size": 1024})
			},
			expectedErr: nil,
		},
		{
			name:         "Object not found (404)",
			expectedSize: 1024,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedErr: storage.ErrObjectNotFound,
		},
		{
			name:         "Size mismatch",
			expectedSize: 1024,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]int64{"size": 2048})
			},
			expectedErr: storage.ErrSizeMismatch,
		},
		{
			name:         "Server 500 error",
			expectedSize: 1024,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("backend failure"))
			},
			expectedErr: errors.New("supabase object info returned 500"),
		},
		{
			name:         "Malformed JSON response",
			expectedSize: 1024,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{invalid-json"))
			},
			expectedErr: errors.New("failed to decode object info"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			client := storage.NewClient(server.URL, "test-key")
			err := client.VerifyObject(context.Background(), "media", "u/123/file.jpg", tc.expectedSize)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.expectedErr)
				}
				if errors.Is(tc.expectedErr, storage.ErrObjectNotFound) || errors.Is(tc.expectedErr, storage.ErrSizeMismatch) {
					if !errors.Is(err, tc.expectedErr) {
						t.Errorf("expected error %v, got %v", tc.expectedErr, err)
					}
				} else if !containsString(err.Error(), tc.expectedErr.Error()) {
					t.Errorf("expected error containing %q, got %v", tc.expectedErr.Error(), err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestStorage_DeleteObjects(t *testing.T) {
	t.Run("Empty slice returns immediately without HTTP call", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		defer server.Close()

		client := storage.NewClient(server.URL, "test-key")
		err := client.DeleteObjects(context.Background(), "media", []string{})
		if err != nil {
			t.Fatalf("unexpected error on empty delete: %v", err)
		}
		if called {
			t.Error("expected no HTTP call for empty keys")
		}
	})

	t.Run("Success on StatusOK, StatusNoContent and StatusNotFound", func(t *testing.T) {
		for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusNotFound} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("expected DELETE method, got %s", r.Method)
				}
				w.WriteHeader(status)
			}))

			client := storage.NewClient(server.URL, "test-key")
			err := client.DeleteObjects(context.Background(), "media", []string{"u/123/a.jpg", "u/123/b.jpg"})
			server.Close()

			if err != nil {
				t.Errorf("expected status %d to succeed, got %v", status, err)
			}
		}
	})

	t.Run("Fails on StatusInternalServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("storage failure"))
		}))
		defer server.Close()

		client := storage.NewClient(server.URL, "test-key")
		err := client.DeleteObjects(context.Background(), "media", []string{"u/123/a.jpg"})
		if err == nil {
			t.Fatal("expected error on 500 status, got nil")
		}
		if !containsString(err.Error(), "supabase delete objects returned 500") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Fails on context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := storage.NewClient(server.URL, "test-key")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		err := client.DeleteObjects(ctx, "media", []string{"u/123/a.jpg"})
		if err == nil {
			t.Fatal("expected error on timeout, got nil")
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsSubstr(s, substr)))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
