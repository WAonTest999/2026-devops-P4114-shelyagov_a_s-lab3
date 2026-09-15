package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
)

// This test needs a disposable PostgreSQL database: CI provides a fresh service.
func TestPostgresConcurrentBooking(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = initialize(context.Background(), db, "admin", "integration-password"); e != nil {
		t.Fatal(e)
	}
	a := newApp(db, false)
	// Unique fixtures also allow repeated runs without clearing user data.
	var userID, eventID int64
	e = db.QueryRow("INSERT INTO users(login,password_hash) VALUES('integration-' || gen_random_uuid(), 'unused') RETURNING id").Scan(&userID)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Exec("DELETE FROM users WHERE id=$1", userID)
	e = db.QueryRow("INSERT INTO events(title,starts_at,rows,cols) VALUES('Race',NOW()+INTERVAL '1 day',10,10) RETURNING id").Scan(&eventID)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Exec("DELETE FROM events WHERE id=$1", eventID)
	_, e = db.Exec("INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,NOW()+INTERVAL '1 hour') ON CONFLICT(token_hash) DO UPDATE SET user_id=$2,expires_at=NOW()+INTERVAL '1 hour'", hashToken("test"), userID)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- request(a, "POST", fmt.Sprintf("/api/events/%d/bookings", eventID), `{"seats":[1]}`, true).Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("one winner expected: %v", counts)
	}
	// A multi-seat conflict must roll back the otherwise available seat too.
	status(t, request(a, "POST", fmt.Sprintf("/api/events/%d/bookings", eventID), `{"seats":[2,1]}`, true), 409)
	var count int
	if e = db.QueryRow("SELECT COUNT(*) FROM bookings WHERE event_id=$1", eventID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("atomic rollback failed: count=%d err=%v", count, e)
	}
}
