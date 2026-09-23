package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hackathon/backend/internal/domain"
)

//go:embed migrations/001_sessions.sql
var migration string

var ErrNotFound = errors.New("session not found")

type Store interface {
	Save(context.Context, *domain.Session) error
	Load(context.Context, string) (*domain.Session, error)
	List(context.Context) ([]domain.Session, error)
	Close()
}
type Memory struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewMemory() *Memory { return &Memory{data: map[string][]byte{}} }
func (m *Memory) Save(_ context.Context, s *domain.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.data) >= 1000 {
		if _, ok := m.data[s.ID]; !ok {
			return errors.New("demo session capacity reached")
		}
	}
	b, e := json.Marshal(s)
	m.data[s.ID] = b
	return e
}
func (m *Memory) Load(_ context.Context, id string) (*domain.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.data[id]
	if !ok {
		return nil, ErrNotFound
	}
	var s domain.Session
	err := json.Unmarshal(b, &s)
	return &s, err
}
func (m *Memory) List(_ context.Context) ([]domain.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Session{}
	for _, b := range m.data {
		var s domain.Session
		if json.Unmarshal(b, &s) == nil {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *Memory) Close() {}

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	_, err = p.Exec(ctx, migration)
	if err != nil {
		p.Close()
		return nil, err
	}
	return &Postgres{p}, nil
}
func (p *Postgres) Save(ctx context.Context, s *domain.Session) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	_, e = p.pool.Exec(ctx, `INSERT INTO voice_sessions(id,payload) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET payload=EXCLUDED.payload,updated_at=now()`, s.ID, b)
	return e
}
func (p *Postgres) Load(ctx context.Context, id string) (*domain.Session, error) {
	var b []byte
	err := p.pool.QueryRow(ctx, `SELECT payload FROM voice_sessions WHERE id=$1`, id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var s domain.Session
	err = json.Unmarshal(b, &s)
	return &s, err
}
func (p *Postgres) List(ctx context.Context) ([]domain.Session, error) {
	rows, e := p.pool.Query(ctx, `SELECT payload FROM voice_sessions ORDER BY updated_at DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Session{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		var s domain.Session
		if e = json.Unmarshal(b, &s); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (p *Postgres) Close() { p.pool.Close() }
