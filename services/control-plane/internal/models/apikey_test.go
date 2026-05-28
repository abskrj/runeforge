package models_test

import (
	"testing"

	"github.com/abskrj/velane/services/control-plane/internal/models"
)

func TestAPIKey_HasScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		check  string
		want   bool
	}{
		{
			name:   "direct match invoke",
			scopes: []string{"invoke"},
			check:  "invoke",
			want:   true,
		},
		{
			name:   "direct match manage",
			scopes: []string{"manage"},
			check:  "manage",
			want:   true,
		},
		{
			name:   "admin grants any scope",
			scopes: []string{"admin"},
			check:  "invoke",
			want:   true,
		},
		{
			name:   "admin grants manage",
			scopes: []string{"admin"},
			check:  "manage",
			want:   true,
		},
		{
			name:   "missing scope returns false",
			scopes: []string{"invoke"},
			check:  "manage",
			want:   false,
		},
		{
			name:   "empty scopes returns false",
			scopes: []string{},
			check:  "invoke",
			want:   false,
		},
		{
			name:   "multiple scopes, one matches",
			scopes: []string{"invoke", "manage"},
			check:  "manage",
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := &models.APIKey{Scopes: tc.scopes}
			got := key.HasScope(tc.check)
			if got != tc.want {
				t.Errorf("HasScope(%q) with scopes %v = %v; want %v", tc.check, tc.scopes, got, tc.want)
			}
		})
	}
}
