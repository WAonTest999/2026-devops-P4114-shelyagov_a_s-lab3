package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db     *sql.DB
	secure bool
}
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Admin bool   `json:"admin"`
}
type Event struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	Rows     int       `json:"rows"`
	Cols     int       `json:"cols"`
}
type Booking struct {
	ID   int64 `json:"id"`
	Seat int   `json:"seat"`
	Mine bool  `json:"mine"`
}

func newApp(db *sql.DB, secure bool) *App {
	return &App{db: db, secure: secure}
}

type response struct {
	http.ResponseWriter
	status int
}

func (w *response) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (a *App) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /api/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := a.db.PingContext(r.Context()); err != nil {
			fail(w, 503, "База данных недоступна")
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	m.HandleFunc("POST /api/register", a.register)
	m.HandleFunc("POST /api/login", a.login)
	m.HandleFunc("POST /api/logout", a.logout)
	m.HandleFunc("GET /api/me", a.me)
	m.HandleFunc("GET /api/events", a.events)
	m.HandleFunc("POST /api/events", a.createEvent)
	m.HandleFunc("PUT /api/events/{id}", a.updateEvent)
	m.HandleFunc("DELETE /api/events/{id}", a.deleteEvent)
	m.HandleFunc("GET /api/events/{id}/bookings", a.bookings)
	m.HandleFunc("POST /api/events/{id}/bookings", a.reserve)
	m.HandleFunc("DELETE /api/bookings/{id}", a.cancelBooking)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					fail(w, 403, "Недопустимый источник запроса")
					return
				}
			}
		}
		start := time.Now()
		out := &response{ResponseWriter: w, status: 200}
		m.ServeHTTP(out, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}

		log.Printf("method=%s route=%q status=%d duration=%s", r.Method, route, out.status, time.Since(start))
	})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "Ожидается JSON")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "Некорректный JSON")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		fail(w, 400, "Лишние данные в запросе")
		return false
	}
	return true
}
func credentials(login, password string) error {
	if len(login) < 3 || len(login) > 40 || strings.TrimSpace(login) != login || len(password) < 8 || len(password) > 72 {
		return errors.New("Логин: 3–40 символов; пароль: 8–72 байта")
	}
	return nil
}
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func (a *App) user(r *http.Request) (User, error) {
	var u User
	c, e := r.Cookie("session")
	if e != nil {
		return u, e
	}
	e = a.db.QueryRowContext(r.Context(), "SELECT u.id,u.login,u.admin FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>NOW()", hashToken(c.Value)).Scan(&u.ID, &u.Login, &u.Admin)
	return u, e
}
func (a *App) require(w http.ResponseWriter, r *http.Request, admin bool) (User, bool) {
	u, e := a.user(r)
	if e != nil {
		fail(w, 401, "Войдите в аккаунт")
		return u, false
	}
	if admin && !u.Admin {
		fail(w, 403, "Доступ только администратору")
		return u, false
	}
	return u, true
}
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if e := credentials(in.Login, in.Password); e != nil {
		fail(w, 400, e.Error())
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if e != nil {
		fail(w, 500, "Ошибка пароля")
		return
	}
	_, e = a.db.ExecContext(r.Context(), "INSERT INTO users(login,password_hash) VALUES($1,$2)", in.Login, string(hash))
	if e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) && pg.Code == "23505" {
			fail(w, 409, "Логин уже занят")
		} else {
			fail(w, 500, "Не удалось создать аккаунт")
		}
		return
	}
	write(w, 201, map[string]string{"status": "created"})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if credentials(in.Login, in.Password) != nil {
		fail(w, 401, "Неверный логин или пароль")
		return
	}
	var u User
	var hash string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,login,admin,password_hash FROM users WHERE login=$1", in.Login).Scan(&u.ID, &u.Login, &u.Admin, &hash)
	if e != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		fail(w, 401, "Неверный логин или пароль")
		return
	}
	raw := make([]byte, 32)
	if _, e = rand.Read(raw); e != nil {
		fail(w, 500, "Ошибка сессии")
		return
	}
	token := hex.EncodeToString(raw)
	_, e = a.db.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)", hashToken(token), u.ID, time.Now().Add(24*time.Hour))
	if e != nil {
		fail(w, 500, "Ошибка сессии")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	write(w, 200, u)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("session"); e == nil {
		if _, e = a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hashToken(c.Value)); e != nil {
			fail(w, 500, "Ошибка выхода")
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode})
	write(w, 200, map[string]string{"status": "ok"})
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	if u, ok := a.require(w, r, false); ok {
		write(w, 200, u)
	}
}
func validEvent(e Event) bool {
	return strings.TrimSpace(e.Title) != "" && len(e.Title) <= 200 && !e.StartsAt.IsZero() && e.Rows >= 1 && e.Rows <= 20 && e.Cols >= 1 && e.Cols <= 20
}
func id(w http.ResponseWriter, r *http.Request) (int64, bool) {
	n, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || n < 1 {
		fail(w, 400, "Некорректный идентификатор")
		return 0, false
	}
	return n, true
}
func (a *App) events(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,title,starts_at,rows,cols FROM events ORDER BY starts_at")
	if e != nil {
		fail(w, 500, "Ошибка чтения концертов")
		return
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.Title, &v.StartsAt, &v.Rows, &v.Cols); e != nil {
			fail(w, 500, "Ошибка чтения концерта")
			return
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		fail(w, 500, "Ошибка чтения концертов")
		return
	}
	write(w, 200, out)
}
func (a *App) createEvent(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.require(w, r, true); !ok {
		return
	}
	var v Event
	if !decode(w, r, &v) {
		return
	}
	if !validEvent(v) {
		fail(w, 400, "Проверьте название, дату и размер зала (1–20)")
		return
	}
	e := a.db.QueryRowContext(r.Context(), "INSERT INTO events(title,starts_at,rows,cols) VALUES($1,$2,$3,$4) RETURNING id", v.Title, v.StartsAt, v.Rows, v.Cols).Scan(&v.ID)
	if e != nil {
		fail(w, 500, "Ошибка создания концерта")
		return
	}
	write(w, 201, v)
}
func (a *App) updateEvent(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.require(w, r, true); !ok {
		return
	}
	n, ok := id(w, r)
	if !ok {
		return
	}
	var v Event
	if !decode(w, r, &v) {
		return
	}
	if !validEvent(v) {
		fail(w, 400, "Некорректный концерт")
		return
	}
	// Hall layout stays fixed so existing tickets never change meaning.
	res, e := a.db.ExecContext(r.Context(), "UPDATE events SET title=$1,starts_at=$2 WHERE id=$3 AND rows=$4 AND cols=$5", v.Title, v.StartsAt, n, v.Rows, v.Cols)
	if e != nil {
		fail(w, 500, "Ошибка изменения концерта")
		return
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		fail(w, 409, "Концерт не найден или изменён размер зала")
		return
	}
	v.ID = n
	write(w, 200, v)
}
func (a *App) deleteEvent(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.require(w, r, true); !ok {
		return
	}
	n, ok := id(w, r)
	if !ok {
		return
	}
	res, e := a.db.ExecContext(r.Context(), "DELETE FROM events WHERE id=$1", n)
	if e != nil {
		fail(w, 500, "Ошибка удаления концерта")
		return
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		fail(w, 404, "Концерт не найден")
		return
	}
	write(w, 200, map[string]string{"status": "deleted"})
}
func (a *App) bookings(w http.ResponseWriter, r *http.Request) {
	n, ok := id(w, r)
	if !ok {
		return
	}
	u, _ := a.user(r)
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,seat,user_id=$2 FROM bookings WHERE event_id=$1 ORDER BY seat", n, u.ID)
	if e != nil {
		fail(w, 500, "Ошибка чтения брони")
		return
	}
	defer rows.Close()
	out := []Booking{}
	for rows.Next() {
		var b Booking
		if e = rows.Scan(&b.ID, &b.Seat, &b.Mine); e != nil {
			fail(w, 500, "Ошибка чтения брони")
			return
		}
		if !b.Mine {
			b.ID = 0
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		fail(w, 500, "Ошибка чтения брони")
		return
	}
	write(w, 200, out)
}
func validateSeats(seats []int, max int) error {
	if len(seats) < 1 || len(seats) > 10 {
		return errors.New("Выберите от 1 до 10 мест")
	}
	seen := map[int]bool{}
	for _, s := range seats {
		if s < 1 || s > max || seen[s] {
			return errors.New("Некорректные или повторяющиеся места")
		}
		seen[s] = true
	}
	return nil
}
func (a *App) reserve(w http.ResponseWriter, r *http.Request) {
	u, ok := a.require(w, r, false)
	if !ok {
		return
	}
	n, ok := id(w, r)
	if !ok {
		return
	}
	var in struct {
		Seats []int `json:"seats"`
	}
	if !decode(w, r, &in) {
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, 500, "Ошибка бронирования")
		return
	}
	defer tx.Rollback()
	var max int
	var when time.Time
	e = tx.QueryRowContext(r.Context(), "SELECT rows*cols,starts_at FROM events WHERE id=$1 FOR UPDATE", n).Scan(&max, &when)
	if e != nil {
		fail(w, 404, "Концерт не найден")
		return
	}
	if !when.After(time.Now()) {
		fail(w, 400, "Бронирование закрыто")
		return
	}
	if e = validateSeats(in.Seats, max); e != nil {
		fail(w, 400, e.Error())
		return
	}
	for _, seat := range in.Seats {
		_, e = tx.ExecContext(r.Context(), "INSERT INTO bookings(event_id,user_id,seat) VALUES($1,$2,$3)", n, u.ID, seat)
		if e != nil {
			var pg *pgconn.PgError
			if errors.As(e, &pg) && pg.Code == "23505" {
				fail(w, 409, "Одно из мест уже занято. Обновите зал")
			} else {
				fail(w, 500, "Ошибка бронирования")
			}
			return
		}
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, "Ошибка сохранения брони")
		return
	}
	write(w, 201, map[string]string{"status": "booked"})
}
func (a *App) cancelBooking(w http.ResponseWriter, r *http.Request) {
	u, ok := a.require(w, r, false)
	if !ok {
		return
	}
	n, ok := id(w, r)
	if !ok {
		return
	}
	res, e := a.db.ExecContext(r.Context(), "DELETE FROM bookings WHERE id=$1 AND user_id=$2", n, u.ID)
	if e != nil {
		fail(w, 500, "Ошибка отмены")
		return
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		fail(w, 404, "Ваша бронь не найдена")
		return
	}
	write(w, 200, map[string]string{"status": "cancelled"})
}
