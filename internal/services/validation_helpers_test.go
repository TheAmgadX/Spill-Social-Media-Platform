package services_test

import (
	"strings"
	"testing"
	"time"

	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/services"
	"github.com/TheAmgadX/Spill-Social-Media-Platform/internal/views"
	"github.com/google/uuid"
)

func TestValidationHelpers_ValidateUUID(t *testing.T) {
	validUUID := uuid.NewString()

	tests := []struct {
		name      string
		id        string
		fieldName string
		wantErr   bool
	}{
		{"Valid standard UUID", validUUID, "userID", false},
		{"Empty string", "", "userID", true},
		{"Whitespace only", "   ", "userID", true},
		{"Leading whitespace", " " + validUUID, "userID", true},
		{"Trailing whitespace", validUUID + " ", "userID", true},
		{"Nil UUID", "00000000-0000-0000-0000-000000000000", "userID", true},
		{"Malformed short string", "not-a-uuid", "userID", true},
		{"Malformed length", validUUID[:35], "userID", true},
		{"Invalid hex characters", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz", "userID", true},
		{"Contains null byte", validUUID[:10] + "\x00" + validUUID[11:], "userID", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := services.ValidateUUID(tt.id, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidationHelpers_ValidateOptionalUUID(t *testing.T) {
	validUUID := uuid.NewString()
	invalid := "not-a-uuid"
	empty := ""

	if err := services.ValidateOptionalUUID(nil, "field"); err != nil {
		t.Errorf("expected nil error for nil pointer, got %v", err)
	}
	if err := services.ValidateOptionalUUID(&empty, "field"); err != nil {
		t.Errorf("expected nil error for empty string, got %v", err)
	}
	if err := services.ValidateOptionalUUID(&validUUID, "field"); err != nil {
		t.Errorf("expected nil error for valid UUID, got %v", err)
	}
	if err := services.ValidateOptionalUUID(&invalid, "field"); err == nil {
		t.Errorf("expected error for invalid UUID, got nil")
	}
}

func TestValidationHelpers_ValidateCleanString(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		allowNewlines bool
		wantErr       bool
	}{
		{"Valid plain ASCII", "Hello world 123!", false, false},
		{"Valid multi-byte Arabic", "مرحبا بك في منصة سبيل", false, false},
		{"Valid multi-byte Emojis", "Hello 👋 🌍 🎉", false, false},
		{"Contains null byte", "hello\x00world", false, true},
		{"Contains newline with allowNewlines=false", "hello\nworld", false, true},
		{"Contains newline with allowNewlines=true", "hello\nworld", true, false},
		{"Contains tab with allowNewlines=true", "hello\tworld", true, false},
		{"Contains carriage return with allowNewlines=true", "hello\r\nworld", true, false},
		{"Contains bell control char with allowNewlines=true", "hello\x07world", true, true},
		{"Invalid UTF-8 sequence", "hello\xff\xfe\xfdworld", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := services.ValidateCleanString(tt.input, "testField", tt.allowNewlines)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCleanString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidationHelpers_ValidateRuneLength(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		min     int
		max     int
		wantErr bool
	}{
		{"Exact min length", "abc", 3, 10, false},
		{"Exact max length", "abcdefghij", 3, 10, false},
		{"Between bounds", "hello", 3, 10, false},
		{"Below min length", "ab", 3, 10, true},
		{"Above max length", "abcdefghijk", 3, 10, true},
		{"Multi-byte runes exact length", "مرحبا", 5, 5, false}, // 5 Arabic runes
		{"Emoji runes exact length", "🎉🎊🎈", 3, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := services.ValidateRuneLength(tt.input, tt.min, tt.max, "field")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRuneLength(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidationHelpers_ValidateEmail(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantEmail string
		wantErr   bool
	}{
		{"Valid standard email", "user@example.com", "user@example.com", false},
		{"Trimming and lowercasing", "  USER@EXAMPLE.COM  ", "user@example.com", false},
		{"Valid subdomain", "alice@mail.example.org", "alice@mail.example.org", false},
		{"Empty email", "", "", true},
		{"Whitespace only", "   \t\n  ", "", true},
		{"Missing @", "userexample.com", "", true},
		{"Missing domain", "user@", "", true},
		{"Missing TLD", "user@localhost", "", true},
		{"Consecutive dots in domain", "user@example..com", "", true},
		{"Leading dot in domain", "user@.example.com", "", true},
		{"Trailing dot in domain", "user@example.com.", "", true},
		{"Disposable domain mailinator", "hacker@mailinator.com", "", true},
		{"Disposable domain 10minutemail", "temp@10minutemail.com", "", true},
		{"Contains null byte", "user\x00@example.com", "", true},
		{"Contains newline", "user\n@example.com", "", true},
		{"Display name syntax rejected", "User <user@example.com>", "", true},
		{"Max length 254 chars", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 185) + ".com", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 185) + ".com", false},
		{"Exceeds max length (255 chars)", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 186) + ".com", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := services.ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
			if !tt.wantErr && res != tt.wantEmail {
				t.Errorf("ValidateEmail(%q) = %q, want %q", tt.email, res, tt.wantEmail)
			}
		})
	}
}

func TestValidationHelpers_ValidateUsername(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		forSignup    bool
		wantUsername string
		wantErr      bool
	}{
		{"Valid 3 chars", "abc", true, "abc", false},
		{"Valid 20 chars", "abcdefghijklmnopqrst", true, "abcdefghijklmnopqrst", false},
		{"Valid alphanumeric with underscore", "john_doe_123", true, "john_doe_123", false},
		{"Too short (2 chars)", "ab", true, "", true},
		{"Too long (21 chars)", "abcdefghijklmnopqrstu", true, "", true},
		{"Contains uppercase", "JohnDoe", true, "", true},
		{"Contains hyphen", "john-doe", true, "", true},
		{"Contains dot", "john.doe", true, "", true},
		{"Contains space", "john doe", true, "", true},
		{"Contains null byte", "john\x00doe", true, "", true},
		{"Empty username", "", true, "", true},
		{"Leading underscore on signup", "_johndoe", true, "", true},
		{"Trailing underscore on signup", "johndoe_", true, "", true},
		{"Reserved username admin on signup", "admin", true, "", true},
		{"Reserved username root on signup", "root", true, "", true},
		{"Reserved username allowed on profile lookup", "admin", false, "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res string
			var err error
			if tt.forSignup {
				res, err = services.ValidateSignupUsername(tt.username)
			} else {
				res, err = services.ValidateUsernameFormat(tt.username)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername(%q, signup=%v) error = %v, wantErr %v", tt.username, tt.forSignup, err, tt.wantErr)
			}
			if !tt.wantErr && res != tt.wantUsername {
				t.Errorf("ValidateUsername(%q) = %q, want %q", tt.username, res, tt.wantUsername)
			}
		})
	}
}

func TestValidationHelpers_ValidateDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"Exact min 1 char", "A", "A", false},
		{"Exact max 50 chars", strings.Repeat("x", 50), strings.Repeat("x", 50), false},
		{"Arabic display name", "أحمد محمد علي", "أحمد محمد علي", false},
		{"Emoji display name", "Jane Doe 🌟", "Jane Doe 🌟", false},
		{"Trimmed with whitespace around", "  Cool User  ", "Cool User", false},
		{"Empty display name", "", "", true},
		{"Whitespace only", "   \t  ", "", true},
		{"Exceeds 50 chars (51 runes)", strings.Repeat("x", 51), "", true},
		{"Contains newline", "Line1\nLine2", "", true},
		{"Contains null byte", "User\x00Name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := services.ValidateDisplayName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDisplayName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && res != tt.want {
				t.Errorf("ValidateDisplayName(%q) = %q, want %q", tt.input, res, tt.want)
			}
		})
	}
}

func TestValidationHelpers_ValidatePagination(t *testing.T) {
	validRecordID := uuid.NewString()
	validCursor := views.EncodeCursor(time.Now(), validRecordID)
	invalidCursorUUID := views.EncodeCursor(time.Now(), "not-a-uuid")

	tests := []struct {
		name      string
		limit     int
		cursorStr string
		maxLimit  int
		wantErr   bool
	}{
		{"Valid limit lower bound", 1, "", 100, false},
		{"Valid limit upper bound", 100, "", 100, false},
		{"Valid limit default 20", 20, "", 100, false},
		{"Valid cursor passed", 10, validCursor, 100, false},
		{"Negative limit", -1, "", 100, true},
		{"Zero limit", 0, "", 100, true},
		{"Limit exceeds max (101)", 101, "", 100, true},
		{"Huge limit overflow (999999)", 999999, "", 100, true},
		{"Malformed base64 cursor", 10, "!!!bad-base64", 100, true},
		{"Malformed ID in cursor", 10, invalidCursorUUID, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, cTime, cID, err := services.ValidatePagination(tt.limit, tt.cursorStr, tt.maxLimit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePagination(%d, %q) error = %v, wantErr %v", tt.limit, tt.cursorStr, err, tt.wantErr)
			}
			if !tt.wantErr {
				if limit != tt.limit {
					t.Errorf("limit = %d, want %d", limit, tt.limit)
				}
				if tt.cursorStr != "" && (cTime == nil || cID != validRecordID) {
					t.Errorf("cursor decode failed: time=%v, id=%s", cTime, cID)
				}
			}
		})
	}
}

func TestValidationHelpers_ParseSearchCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  string
		want    int
		wantErr bool
	}{
		{"Empty cursor", "", 0, false},
		{"Plain int 0", "0", 0, false},
		{"Plain int 50", "50", 50, false},
		{"Base64 int 20", "MjA", 20, false},
		{"Negative int", "-5", 0, true},
		{"Malformed string", "not-a-number", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := services.ParseSearchCursor(tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSearchCursor(%q) error = %v, wantErr %v", tt.cursor, err, tt.wantErr)
			}
			if !tt.wantErr && res != tt.want {
				t.Errorf("ParseSearchCursor(%q) = %d, want %d", tt.cursor, res, tt.want)
			}
		})
	}
}

func TestValidationHelpers_ValidateRole(t *testing.T) {
	tests := []struct {
		role    string
		wantErr bool
	}{
		{"user", false},
		{"admin", false},
		{"  user  ", false},
		{"moderator", true},
		{"root", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			_, err := services.ValidateRole(tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRole(%q) error = %v, wantErr %v", tt.role, err, tt.wantErr)
			}
		})
	}
}

func TestValidationHelpers_ValidateFilename(t *testing.T) {
	tests := []struct {
		filename string
		wantErr  bool
	}{
		{"photo.jpg", false},
		{"my-avatar_2026.png", false},
		{"../../etc/passwd", true},
		{"../secrets.env", true},
		{"/root/file.png", true},
		{"C:\\Windows\\system32", true},
		{"file\x00.png", true},
		{"", true},
		{"   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			_, err := services.ValidateFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilename(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
			}
		})
	}
}

func TestValidationHelpers_ValidateURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/images/avatar.jpg", false},
		{"http://localhost:8080/storage/u/123", false},
		{"javascript:alert(1)", true},
		{"data:image/png;base64,iVBORw0KGgo...", true},
		{"file:///etc/passwd", true},
		{"ftp://ftp.example.com", true},
		{"", true},
		{"not-a-url", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := services.ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
