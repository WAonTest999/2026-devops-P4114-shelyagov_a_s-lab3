package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

func setup(t *testing.T) (*App, sqlmock.Sqlmock) {
	t.Helper()
	db, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := m.ExpectationsWereMet(); e != nil {
			t.Error(e)
		}
		db.Close()
	})
	return newApp(db, false), m
}
func request(a *App, method, path, body string, auth bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if auth {
		r.AddCookie(&http.Cookie{Name: "session", Value: "test"})
	}
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d want=%d: %s", w.Code, want, w.Body.String())
	}
}
func auth(m sqlmock.Sqlmock, admin bool) {
	m.ExpectQuery("SELECT u.id").WillReturnRows(sqlmock.NewRows([]string{"id", "login", "admin"}).AddRow(1, "alice", admin))
}

var dbError = errors.New("database unavailable")

func TestReadinessDatabaseFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectPing().WillReturnError(dbError)
	status(t, request(newApp(db, false), "GET", "/api/ready", "", false), 503)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

const eventJSON = `{"title":"Концерт","starts_at":"2099-10-01T12:00:00Z","rows":10,"cols":10}`

func TestValidation(t *testing.T) {
	for _, v := range []struct {
		l, p string
		ok   bool
	}{{"alice", "password123", true}, {"ab", "password123", false}, {"alice", "short", false}, {" alice", "password123", false}, {strings.Repeat("a", 41), "password123", false}, {"alice", strings.Repeat("x", 73), false}} {
		if (credentials(v.l, v.p) == nil) != v.ok {
			t.Error(v)
		}
	}
	for _, s := range [][]int{nil, {0}, {101}, {1, 1}, {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}} {
		if validateSeats(s, 100) == nil {
			t.Error(s)
		}
	}
	if validateSeats([]int{1, 100}, 100) != nil {
		t.Fatal("valid seats rejected")
	}
	if validEvent(Event{}) {
		t.Fatal("empty event")
	}
	if hashToken("x") == hashToken("y") {
		t.Fatal("hash collision")
	}
}
func TestAuth(t *testing.T) {
	a, m := setup(t)
	status(t, request(a, "GET", "/api/me", "", false), 401)
	auth(m, false)
	status(t, request(a, "GET", "/api/me", "", true), 200)
	status(t, request(a, "POST", "/api/register", `{"login":"a","password":"b"}`, false), 400)
	m.ExpectExec("INSERT INTO users").WillReturnResult(sqlmock.NewResult(1, 1))
	status(t, request(a, "POST", "/api/register", `{"login":"alice","password":"password123"}`, false), 201)
	for _, e := range []error{&pgconn.PgError{Code: "23505"}, dbError} {
		m.ExpectExec("INSERT INTO users").WillReturnError(e)
		want := 500
		if e != dbError {
			want = 409
		}
		status(t, request(a, "POST", "/api/register", `{"login":"alice","password":"password123"}`, false), want)
	}
	status(t, request(a, "POST", "/api/login", `{"login":"a","password":"b"}`, false), 401)
	m.ExpectQuery("SELECT id,login").WillReturnError(sql.ErrNoRows)
	status(t, request(a, "POST", "/api/login", `{"login":"alice","password":"password123"}`, false), 401)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	for _, failSession := range []bool{false, true} {
		m.ExpectQuery("SELECT id,login").WillReturnRows(sqlmock.NewRows([]string{"id", "login", "admin", "password_hash"}).AddRow(1, "alice", false, string(hash)))
		x := m.ExpectExec("INSERT INTO sessions")
		if failSession {
			x.WillReturnError(dbError)
		} else {
			x.WillReturnResult(sqlmock.NewResult(1, 1))
		}
		w := request(a, "POST", "/api/login", `{"login":"alice","password":"password123"}`, false)
		if failSession {
			status(t, w, 500)
		} else {
			status(t, w, 200)
			c := w.Result().Cookies()[0]
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || len(c.Value) != 64 {
				t.Fatal(c)
			}
		}
	}
	m.ExpectExec("DELETE FROM sessions").WillReturnResult(sqlmock.NewResult(0, 1))
	status(t, request(a, "POST", "/api/logout", "", true), 200)
	m.ExpectExec("DELETE FROM sessions").WillReturnError(dbError)
	status(t, request(a, "POST", "/api/logout", "", true), 500)
	status(t, request(a, "POST", "/api/logout", "", false), 200)
}
func TestHTTPBoundaries(t *testing.T) {
	a, _ := setup(t)
	status(t, request(a, "GET", "/api/health", "", false), 200)
	status(t, request(a, "GET", "/missing", "", false), 404)
	for _, body := range []string{`{`, `{"unknown":1}`, `{} {}`} {
		status(t, request(a, "POST", "/api/register", body, false), 400)
	}
	r := httptest.NewRequest("POST", "/api/register", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	status(t, w, 415)
	r = httptest.NewRequest("POST", "/api/logout", nil)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	status(t, w, 403)
	status(t, request(a, "GET", "/api/events/no/bookings", "", false), 400)
}
func TestEvents(t *testing.T) {
	a, m := setup(t)
	m.ExpectQuery("SELECT id,title").WillReturnRows(sqlmock.NewRows([]string{"id", "title", "starts_at", "rows", "cols"}).AddRow(1, "Concert", time.Now(), 10, 10))
	status(t, request(a, "GET", "/api/events", "", false), 200)
	m.ExpectQuery("SELECT id,title").WillReturnError(dbError)
	status(t, request(a, "GET", "/api/events", "", false), 500)
	status(t, request(a, "POST", "/api/events", eventJSON, false), 401)
	auth(m, false)
	status(t, request(a, "POST", "/api/events", eventJSON, true), 403)
	auth(m, true)
	status(t, request(a, "POST", "/api/events", `{}`, true), 400)
	auth(m, true)
	m.ExpectQuery("INSERT INTO events").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	status(t, request(a, "POST", "/api/events", eventJSON, true), 201)
	auth(m, true)
	m.ExpectQuery("INSERT INTO events").WillReturnError(dbError)
	status(t, request(a, "POST", "/api/events", eventJSON, true), 500)
	for _, method := range []string{"PUT", "DELETE"} {
		auth(m, true)
		status(t, request(a, method, "/api/events/invalid", eventJSON, true), 400)
	}
	auth(m, true)
	status(t, request(a, "PUT", "/api/events/1", `{}`, true), 400)
	for _, n := range []int64{1, 0, -1} {
		auth(m, true)
		x := m.ExpectExec("UPDATE events")
		want := 200
		if n == -1 {
			x.WillReturnError(dbError)
			want = 500
		} else {
			x.WillReturnResult(sqlmock.NewResult(0, n))
			if n == 0 {
				want = 409
			}
		}
		status(t, request(a, "PUT", "/api/events/1", eventJSON, true), want)
	}
	for _, n := range []int64{1, 0, -1} {
		auth(m, true)
		x := m.ExpectExec("DELETE FROM events")
		want := 200
		if n == -1 {
			x.WillReturnError(dbError)
			want = 500
		} else {
			x.WillReturnResult(sqlmock.NewResult(0, n))
			if n == 0 {
				want = 404
			}
		}
		status(t, request(a, "DELETE", "/api/events/1", "", true), want)
	}
}
func TestBookings(t *testing.T) {
	a, m := setup(t)
	m.ExpectQuery("SELECT id,seat").WillReturnRows(sqlmock.NewRows([]string{"id", "seat", "mine"}).AddRow(5, 1, false).AddRow(6, 2, true))
	w := request(a, "GET", "/api/events/1/bookings", "", false)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), `"id":0`) {
		t.Fatal("private booking id leaked")
	}
	m.ExpectQuery("SELECT id,seat").WillReturnError(dbError)
	status(t, request(a, "GET", "/api/events/1/bookings", "", false), 500)
	status(t, request(a, "POST", "/api/events/1/bookings", `{"seats":[1]}`, false), 401)
	for _, scenario := range []string{"ok", "conflict", "db", "past", "badseat", "missing", "commit", "begin"} {
		t.Run(scenario, func(t *testing.T) {
			a, m := setup(t)
			auth(m, false)
			begin := m.ExpectBegin()
			if scenario == "begin" {
				begin.WillReturnError(dbError)
				status(t, request(a, "POST", "/api/events/1/bookings", `{"seats":[1]}`, true), 500)
				return
			}
			q := m.ExpectQuery("SELECT rows")
			when := time.Now().Add(time.Hour)
			want := 201
			if scenario == "missing" {
				q.WillReturnError(sql.ErrNoRows)
				want = 404
			} else {
				max := 100
				if scenario == "past" {
					when = time.Now().Add(-time.Hour)
					want = 400
				}
				if scenario == "badseat" {
					max = 0
					want = 400
				}
				q.WillReturnRows(sqlmock.NewRows([]string{"max", "starts_at"}).AddRow(max, when))
			}
			if want == 201 {
				x := m.ExpectExec("INSERT INTO bookings")
				switch scenario {
				case "conflict":
					x.WillReturnError(&pgconn.PgError{Code: "23505"})
					want = 409
				case "db":
					x.WillReturnError(dbError)
					want = 500
				default:
					x.WillReturnResult(sqlmock.NewResult(1, 1))
				}
				if want == 201 {
					c := m.ExpectCommit()
					if scenario == "commit" {
						c.WillReturnError(dbError)
						want = 500
					}
				} else {
					m.ExpectRollback()
				}
			} else {
				m.ExpectRollback()
			}
			status(t, request(a, "POST", "/api/events/1/bookings", `{"seats":[1]}`, true), want)
		})
	}
	for _, n := range []int64{1, 0, -1} {
		auth(m, false)
		x := m.ExpectExec("DELETE FROM bookings")
		want := 200
		if n == -1 {
			x.WillReturnError(dbError)
			want = 500
		} else {
			x.WillReturnResult(sqlmock.NewResult(0, n))
			if n == 0 {
				want = 404
			}
		}
		status(t, request(a, "DELETE", "/api/bookings/1", "", true), want)
	}
}
