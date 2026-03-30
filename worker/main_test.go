package main

import "testing"

func TestBuildPostgresConnString(t *testing.T) {
	dsn := buildPostgresConnString("db-host", 5432, "user1", "pass1", "votesdb")
	expected := "host=db-host port=5432 user=user1 password=pass1 dbname=votesdb sslmode=disable"

	if dsn != expected {
		t.Fatalf("unexpected DSN. got=%q want=%q", dsn, expected)
	}
}
