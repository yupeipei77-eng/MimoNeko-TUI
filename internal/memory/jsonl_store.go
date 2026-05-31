package memory

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

const defaultSearchLimit = 8

// JSONLStore is a local-first append-only durable memory store.
type JSONLStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONLStore(path string) *JSONLStore {
	return &JSONLStore{path: path}
}

func (s *JSONLStore) Put(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	record.Text = strings.TrimSpace(record.Text)
	if record.Text == "" {
		return fmt.Errorf("memory text is required")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(record.ID) == "" {
		record.ID = "mem_" + randomHex(8)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(record); err != nil {
		return err
	}
	return file.Sync()
}

func (s *JSONLStore) Get(ctx context.Context, id string) (Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, false, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Record{}, false, nil
	}
	records, err := s.latestRecords(ctx)
	if err != nil {
		return Record{}, false, err
	}
	record, ok := records[id]
	return record, ok, nil
}

func (s *JSONLStore) Search(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	terms := tokenize(query.Text)
	if len(terms) == 0 {
		return nil, nil
	}
	records, err := s.latestRecords(ctx)
	if err != nil {
		return nil, err
	}
	scope := strings.TrimSpace(query.Scope)
	var results []SearchResult
	for _, record := range records {
		if scope != "" && record.Scope != scope {
			continue
		}
		score := scoreRecord(record, terms, query.Text)
		if score <= 0 {
			continue
		}
		results = append(results, SearchResult{Record: record, Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Record.UpdatedAt.After(results[j].Record.UpdatedAt)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *JSONLStore) latestRecords(ctx context.Context) (map[string]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Record{}, nil
		}
		return nil, err
	}
	defer file.Close()

	records := make(map[string]Record)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if strings.TrimSpace(record.ID) == "" {
			continue
		}
		current, ok := records[record.ID]
		if !ok || record.UpdatedAt.After(current.UpdatedAt) {
			records[record.ID] = record
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func scoreRecord(record Record, terms []string, rawQuery string) float64 {
	text := strings.ToLower(record.Text + " " + metadataText(record.Metadata))
	recordTerms := tokenize(text)
	if len(recordTerms) == 0 {
		return 0
	}
	frequencies := make(map[string]int, len(recordTerms))
	for _, term := range recordTerms {
		frequencies[term]++
	}
	score := 0.0
	for _, term := range terms {
		if count := frequencies[term]; count > 0 {
			score += 1 + float64(count-1)*0.2
		}
	}
	query := strings.ToLower(strings.TrimSpace(rawQuery))
	if query != "" && strings.Contains(strings.ToLower(record.Text), query) {
		score += 2
	}
	if !record.UpdatedAt.IsZero() {
		age := time.Since(record.UpdatedAt)
		if age < 30*24*time.Hour {
			score += 0.25
		}
	}
	return score
}

func metadataText(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	var parts []string
	for key, value := range metadata {
		parts = append(parts, key, value)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var terms []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		term := current.String()
		if len([]rune(term)) > 1 {
			terms = append(terms, term)
		}
		current.Reset()
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return terms
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
