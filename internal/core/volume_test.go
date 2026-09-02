package core

import "testing"

func TestNormalizePostgresVersion(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", DefaultPostgresVersion},
		{"16", "16"},
		{" 17 ", "17"},
		{"99", DefaultPostgresVersion},
		{"latest", DefaultPostgresVersion},
	}
	for _, tc := range cases {
		if got := NormalizePostgresVersion(tc.in); got != tc.want {
			t.Fatalf("NormalizePostgresVersion(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDeriveVolumeName(t *testing.T) {
	t.Parallel()
	if got := DeriveVolumeName("postgres", "shop", "16"); got != "pgdata_shop_16" {
		t.Fatalf("got %q", got)
	}
	if got := DeriveVolumeName("postgres", "shop", ""); got != "pgdata_shop_18" {
		t.Fatalf("empty version should normalize, got %q", got)
	}
	if got := DeriveVolumeName("sqlserver", "shop", "16"); got != "sqlserver_shop" {
		t.Fatalf("sqlserver must ignore version, got %q", got)
	}
	if got := DeriveVolumeName("postgres", "My Shop App", "16"); got != "pgdata_my_shop_app_16" {
		t.Fatalf("spaces/case should snake_case, got %q", got)
	}
}

func TestSanitizeIdent(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", ""},
		{"  Shop  ", "shop"},
		{"My Shop", "my_shop"},
		{"my   shop  app", "my_shop_app"},
		{"already_ok", "already_ok"},
		{"My-Shop", "my_shop"},
		{"PG-My__Shop", "pg_my_shop"},
	}
	for _, tc := range cases {
		if got := SanitizeIdent(tc.in); got != tc.want {
			t.Fatalf("SanitizeIdent(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
