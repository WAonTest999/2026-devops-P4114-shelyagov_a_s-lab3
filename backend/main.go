package main

import (
	"context"
	"database/sql"
	_ "embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql
var schema string

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = initialize(ctx, db, os.Getenv("ADMIN_LOGIN"), os.Getenv("ADMIN_PASSWORD")); err != nil {
		log.Fatal(err)
	}
	app := newApp(db, os.Getenv("COOKIE_SECURE") == "true")
	server := &http.Server{Addr: ":8080", Handler: app.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go func() {
		<-stop.Done()
		c, close := context.WithTimeout(context.Background(), 10*time.Second)
		defer close()
		_ = server.Shutdown(c)
	}()
	log.Print("server listening on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func initialize(ctx context.Context, db *sql.DB, login, password string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize schema/bootstrap across replicas.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(741829)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	if login != "" && password != "" {
		if err = credentials(login, password); err != nil {
			return err
		}
		hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO users(login,password_hash,admin) VALUES($1,$2,true) ON CONFLICT(login) DO NOTHING", login, string(hash)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
