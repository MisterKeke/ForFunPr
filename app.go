package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "modernc.org/sqlite"
)

// App struct holds runtime context and is bound to the frontend by Wails.
type App struct {
	ctx context.Context

	db *sql.DB
}

func NewApp() *App {
	return &App{}
}

type favoriteRate struct {
	Code  string  `json:"code"`
	Base  string  `json:"base"`
	To    string  `json:"to"`
	Rate  float64 `json:"rate"`
	Found bool    `json:"found"`
}

type favoritesPayload struct {
	Base      string         `json:"base"`
	Favorites []favoriteRate `json:"favorites"`
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	db, err := sql.Open("sqlite", "database.db")
	if err != nil {
		panic(err)
	}

	a.db = db

	_, err = a.db.Exec(`
        CREATE TABLE IF NOT EXISTS favorite_rates (
            base TEXT NOT NULL,
            quote TEXT NOT NULL,
            PRIMARY KEY (base, quote)
        )
    `)
	if err != nil {
		panic(err)
	}

	// Create todos table if not exists
	_, err = a.db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			is_completed INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			due_date DATE,
			priority TEXT DEFAULT 'medium' CHECK(priority IN ('low','medium','high'))
		)
	`)
	if err != nil {
		panic(err)
	}

	// Create telegram_favorites table if not exists
	_, err = a.db.Exec(`
		CREATE TABLE IF NOT EXISTS telegram_favorites (
			username TEXT PRIMARY KEY,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		panic(err)
	}
}

// --- Новые структуры под формат API v2 ---

// V2SingleRateResponse для эндпоинта /v2/rate/{base}/{quote}
type V2SingleRateResponse struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

// V2RatesResponse для эндпоинта /v2/rates
type V2RatesResponse struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

// --- Измененные структуры для фронтенда (сохраняем обратную совместимость) ---

type RateResult struct {
	Base  string  `json:"base"`
	Date  string  `json:"date"`
	To    string  `json:"to"`
	Rate  float64 `json:"rate"`
	Found bool    `json:"found"`
}

type AllRatesResult struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
	Codes []string           `json:"codes"`
}

// GetRate теперь отправляет точечный запрос к v2 для одной пары.
// Пример: https://frankfurter.dev
// GetRate теперь отправляет точечный запрос к v2 для одной пары.
// Пример: https://frankfurter.dev
func (a *App) GetRate(base string, target string) (*RateResult, error) {
	base = strings.ToUpper(strings.TrimSpace(base))
	target = strings.ToUpper(strings.TrimSpace(target))

	if base == "" || target == "" {
		return nil, fmt.Errorf("base and target currency codes are required")
	}

	if base == target {
		return &RateResult{Base: base, Date: "", To: target, Rate: 1.0, Found: true}, nil
	}

	// ИСПРАВЛЕНО: Безопасное конструирование URL с правильным хостом api.frankfurter.dev
	url := "https://api.frankfurter.dev/v2/rate/" + base + "/" + target

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &RateResult{Base: base, To: target, Found: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var data V2SingleRateResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &RateResult{
		Base:  data.Base,
		Date:  data.Date,
		To:    data.Quote,
		Rate:  data.Rate,
		Found: true,
	}, nil
}

// GetAllRates отправляет запрос ко всем валютам и трансформирует массив v2 в map для UI.
// Пример: https://api.frankfurter.dev/v2/rates?base=USD
func (a *App) GetAllRates(base string) (*AllRatesResult, error) {
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		return nil, fmt.Errorf("base currency code is required")
	}

	// ИСПРАВЛЕНО: Полностью убран fmt.Sprintf, URL собирается через оператор "+"
	// ИСПРАВЛЕНО: Добавлен корректный поддомен "api."
	url := "https://api.frankfurter.dev/v2/rates?base=" + base

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var list []V2RatesResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(list) == 0 {
		return &AllRatesResult{Base: base, Rates: make(map[string]float64), Codes: []string{}}, nil
	}

	ratesMap := make(map[string]float64)
	codes := make([]string, 0, len(list))
	date := list[0].Date // ИСПРАВЛЕНО: Индекс массива [0] для получения даты

	for _, item := range list {
		ratesMap[item.Quote] = item.Rate
		codes = append(codes, item.Quote)
	}
	sort.Strings(codes)

	return &AllRatesResult{
		Base:  base,
		Date:  date,
		Rates: ratesMap,
		Codes: codes,
	}, nil
}

// --- Заглушки методов для Избранного (без изменений) ---

func normalizeCurrency(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return ""
	}
	return code
}

func normalizeFavoritePair(value string) (string, string, bool) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(value)), ":")
	if len(parts) != 2 {
		return "", "", false
	}

	base := normalizeCurrency(parts[0])
	quote := normalizeCurrency(parts[1])
	if base == "" || quote == "" {
		return "", "", false
	}

	return base, quote, true
}

type AddFavoriteResult struct {
	Pair   string `json:"pair"`
	Added  bool   `json:"added"`
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

func (a *App) AddFavorite(name string) AddFavoriteResult {
	base, quote, ok := normalizeFavoritePair(name)
	if !ok {
		return AddFavoriteResult{
			Error: "invalid currency pair format",
		}
	}

	// Проверяем существование
	var exists bool
	err := a.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM favorite_rates WHERE base = ? AND quote = ?)`,
		base, quote,
	).Scan(&exists)
	if err != nil {
		return AddFavoriteResult{
			Error: err.Error(),
		}
	}

	// Если уже существует
	if exists {
		return AddFavoriteResult{
			Pair:   base + ":" + quote,
			Exists: true,
			Added:  false,
		}
	}

	// Вставляем новую запись
	_, err = a.db.Exec(
		`INSERT INTO favorite_rates (base, quote) VALUES (?, ?)`,
		base, quote,
	)
	if err != nil {
		return AddFavoriteResult{
			Error: err.Error(),
		}
	}

	return AddFavoriteResult{
		Pair:   base + ":" + quote,
		Added:  true,
		Exists: false,
	}
}

func (a *App) RemoveFavorite(name string) string {
	base, quote, ok := normalizeFavoritePair(name)
	if !ok {
		return ""
	}

	_, err := a.db.Exec(
		`DELETE FROM favorite_rates WHERE base = ? AND quote = ?`,
		base,
		quote,
	)
	if err != nil {
		return ""
	}

	return base + ":" + quote
}

func (a *App) ListFavorites() []string {
	rows, err := a.db.Query(`SELECT base, quote FROM favorite_rates ORDER BY base, quote`)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	favorites := []string{}
	for rows.Next() {
		var base string
		var quote string
		if err := rows.Scan(&base, &quote); err == nil {
			favorites = append(favorites, base+":"+quote)
		}
	}

	return favorites
}

func (a *App) GetFavoriteswithRates() string {
	payload := favoritesPayload{
		Base:      "",
		Favorites: make([]favoriteRate, 0),
	}

	for _, key := range a.ListFavorites() {
		base, quote, ok := normalizeFavoritePair(key)
		if !ok {
			continue
		}

		res, err := a.GetRate(base, quote)
		if err != nil || !res.Found {
			payload.Favorites = append(payload.Favorites, favoriteRate{
				Code:  key,
				Base:  base,
				To:    quote,
				Found: false,
			})
			continue
		}
		payload.Favorites = append(payload.Favorites, favoriteRate{
			Code:  key,
			Base:  base,
			To:    quote,
			Rate:  res.Rate,
			Found: true,
		})
	}

	data, _ := json.Marshal(payload)
	return string(data)
}

type ToDoItem struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Time      string `json:"time"`
	Details   string `json:"details"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
	DueDate   string `json:"due_date"`
	Priority  string `json:"priority"`
}

// GetTodos returns all todos from the database
func (a *App) GetTodos() ([]ToDoItem, error) {
	rows, err := a.db.Query(`SELECT id, title, description, is_completed, created_at, due_date, priority FROM todos ORDER BY created_at DESC`)
	if err != nil {
		return []ToDoItem{}, err
	}
	defer rows.Close()

	result := []ToDoItem{}
	for rows.Next() {
		var t ToDoItem
		var isCompleted int
		var title sql.NullString
		var description sql.NullString
		var createdAt sql.NullString
		var dueDate sql.NullString
		var priority sql.NullString

		if err := rows.Scan(&t.ID, &title, &description, &isCompleted, &createdAt, &dueDate, &priority); err != nil {
			continue
		}
		t.Title = title.String
		t.Text = title.String
		t.Details = description.String
		t.Done = isCompleted != 0
		t.CreatedAt = createdAt.String
		t.DueDate = dueDate.String
		t.Priority = priority.String

		result = append(result, t)
	}

	return result, nil
}

// normalizeTodoPriority ensures the priority is one of the allowed values, defaulting to "medium".
func normalizeTodoPriority(priority string) string {
	priority = strings.ToLower(strings.TrimSpace(priority))
	switch priority {
	case "low", "medium", "high":
		return priority
	default:
		return "medium"
	}
}

// CreateTodo inserts a new todo with the provided title, description and priority, and returns the full list
func (a *App) CreateTodo(title string, description string, priority string) ([]ToDoItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return a.GetTodos()
	}

	description = strings.TrimSpace(description)
	priority = normalizeTodoPriority(priority)

	_, err := a.db.Exec(
		`INSERT INTO todos (title, description, is_completed, priority) VALUES (?, ?, 0, ?)`,
		title, description, priority,
	)
	if err != nil {
		return []ToDoItem{}, err
	}

	return a.GetTodos()
}

// UpdateTodo updates the title, description and priority for an existing todo and returns the full list
func (a *App) UpdateTodo(id int, title string, description string, priority string) ([]ToDoItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return a.GetTodos()
	}

	description = strings.TrimSpace(description)
	priority = normalizeTodoPriority(priority)

	_, err := a.db.Exec(
		`UPDATE todos SET title = ?, description = ?, priority = ? WHERE id = ?`,
		title, description, priority, id,
	)
	if err != nil {
		return []ToDoItem{}, err
	}

	return a.GetTodos()
}

// ToggleTodo flips the is_completed flag for a given todo id and returns the full list
func (a *App) ToggleTodo(id int) ([]ToDoItem, error) {
	// Get current value
	var cur int
	err := a.db.QueryRow(`SELECT is_completed FROM todos WHERE id = ?`, id).Scan(&cur)
	if err != nil {
		return a.GetTodos()
	}

	newVal := 1
	if cur != 0 {
		newVal = 0
	}

	_, err = a.db.Exec(`UPDATE todos SET is_completed = ? WHERE id = ?`, newVal, id)
	if err != nil {
		return a.GetTodos()
	}

	return a.GetTodos()
}

// DeleteTodo removes a todo by id and returns the full list
func (a *App) DeleteTodo(id int) ([]ToDoItem, error) {
	_, err := a.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return a.GetTodos()
	}
	return a.GetTodos()
}

type TelegramPost struct {
	Text   string   `json:"text"`
	Images []string `json:"images"`
	Date   string   `json:"date"`
	Views  string   `json:"views"`
	PostID string   `json:"postId"`
}

type TelegramCache struct {
	posts     []TelegramPost
	timestamp time.Time
	mu        sync.RWMutex
}

var telegramCache = &TelegramCache{}

// GetChannelPosts получает посты из публичного Telegram канала
func (a *App) GetChannelPosts(channelUsername string) ([]TelegramPost, error) {
	// Проверяем кэш
	if posts, ok := telegramCache.GetPosts(); ok {
		return posts, nil
	}

	channelUsername = strings.TrimSpace(channelUsername)
	if channelUsername == "" {
		return nil, fmt.Errorf("channel username is required")
	}

	// Убираем @ если есть
	channelUsername = strings.TrimPrefix(channelUsername, "@")

	url := fmt.Sprintf("https://t.me/s/%s", channelUsername)

	// Делаем запрос с User-Agent
	posts, err := fetchTelegramPosts(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel posts: %w", err)
	}

	// Сохраняем в кэш
	telegramCache.SetPosts(posts)

	return posts, nil
}

// GetChannelPostsPaginated получает посты с пагинацией
func (a *App) GetChannelPostsPaginated(channelUsername string, before int) ([]TelegramPost, error) {
	channelUsername = strings.TrimSpace(strings.TrimPrefix(channelUsername, "@"))
	if channelUsername == "" {
		return nil, fmt.Errorf("channel username is required")
	}

	var url string
	if before == 0 {
		url = fmt.Sprintf("https://t.me/s/%s", channelUsername)
	} else {
		url = fmt.Sprintf("https://t.me/s/%s?before=%d", channelUsername, before)
	}

	return fetchTelegramPosts(url)
}

// fetchTelegramPosts выполняет запрос и парсит HTML
func fetchTelegramPosts(url string) ([]TelegramPost, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Важно: добавляем User-Agent чтобы не быть заблокированными
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Парсим HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var posts []TelegramPost

	// Ищем сообщения в веб-версии Telegram
	doc.Find(".tgme_widget_message_wrap").Each(func(i int, s *goquery.Selection) {
		post := TelegramPost{}

		// Текст сообщения
		textElement := s.Find(".tgme_widget_message_text")
		post.Text = strings.TrimSpace(textElement.Text())

		// Изображения
		s.Find(".tgme_widget_message_photo_wrap").Each(func(j int, img *goquery.Selection) {
			style, exists := img.Attr("style")
			if exists {
				url := extractImageURL(style)
				if url != "" {
					post.Images = append(post.Images, url)
				}
			}
		})

		// Также ищем изображения в тегах img
		s.Find(".tgme_widget_message_photo img").Each(func(j int, img *goquery.Selection) {
			src, exists := img.Attr("src")
			if exists && src != "" {
				// Проверяем что это не пустой gif
				if !strings.Contains(src, "data:image") {
					post.Images = append(post.Images, src)
				}
			}
		})

		// Дата
		dateElement := s.Find(".tgme_widget_message_date time")
		if dateElement.Length() > 0 {
			post.Date, _ = dateElement.Attr("datetime")
		}

		// Просмотры
		viewsElement := s.Find(".tgme_widget_message_views")
		post.Views = strings.TrimSpace(viewsElement.Text())

		// ID поста для пагинации
		postID, exists := s.Attr("data-post")
		if exists {
			post.PostID = postID
		}

		// Добавляем только если есть текст или изображения
		if post.Text != "" || len(post.Images) > 0 {
			posts = append(posts, post)
		}
	})
	// Reverse posts so newest (latest) appears first
	for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
		posts[i], posts[j] = posts[j], posts[i]
	}
	return posts, nil
}

// extractImageURL извлекает URL изображения из CSS background-image
func extractImageURL(style string) string {
	// Ищем "background-image:url('...')"
	start := strings.Index(style, "background-image:")
	if start == -1 {
		return ""
	}

	// Ищем URL
	urlStart := strings.Index(style[start:], "url(")
	if urlStart == -1 {
		return ""
	}
	urlStart += start + 4

	// Пропускаем пробелы
	for urlStart < len(style) && style[urlStart] == ' ' {
		urlStart++
	}

	// Проверяем кавычки
	hasQuote := false
	if urlStart < len(style) && (style[urlStart] == '\'' || style[urlStart] == '"') {
		hasQuote = true
		urlStart++
	}

	urlEnd := urlStart
	if hasQuote {
		// Ищем закрывающую кавычку
		for urlEnd < len(style) && style[urlEnd] != '\'' && style[urlEnd] != '"' {
			urlEnd++
		}
	} else {
		// Ищем закрывающую скобку
		for urlEnd < len(style) && style[urlEnd] != ')' {
			urlEnd++
		}
	}

	if urlEnd <= urlStart || urlStart >= len(style) {
		return ""
	}

	return style[urlStart:urlEnd]
}

// --- Методы кэша ---

func (c *TelegramCache) GetPosts() ([]TelegramPost, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Since(c.timestamp) < 5*time.Minute && len(c.posts) > 0 {
		return c.posts, true
	}
	return nil, false
}

func (c *TelegramCache) SetPosts(posts []TelegramPost) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.posts = posts
	c.timestamp = time.Now()
}

func (c *TelegramCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.posts = nil
	c.timestamp = time.Time{}
}

// Очистка кэша Telegram
func (a *App) TelegramCacheClear() {
	telegramCache.Clear()
}

// --- Избранные Telegram-каналы ---

func normalizeTelegramUsername(username string) string {
	username = strings.TrimSpace(username)
	username = strings.TrimPrefix(username, "@")
	return strings.ToLower(username)
}

// AddTelegramFavorite сохраняет канал в списке избранных
func (a *App) AddTelegramFavorite(username string) ([]string, error) {
	username = normalizeTelegramUsername(username)
	if username == "" {
		return a.ListTelegramFavorites()
	}

	_, err := a.db.Exec(
		`INSERT OR IGNORE INTO telegram_favorites (username) VALUES (?)`,
		username,
	)
	if err != nil {
		return nil, err
	}

	return a.ListTelegramFavorites()
}

// RemoveTelegramFavorite удаляет канал из списка избранных
func (a *App) RemoveTelegramFavorite(username string) ([]string, error) {
	username = normalizeTelegramUsername(username)
	if username == "" {
		return a.ListTelegramFavorites()
	}

	_, err := a.db.Exec(`DELETE FROM telegram_favorites WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}

	return a.ListTelegramFavorites()
}

// ListTelegramFavorites возвращает список избранных каналов
func (a *App) ListTelegramFavorites() ([]string, error) {
	rows, err := a.db.Query(`SELECT username FROM telegram_favorites ORDER BY added_at ASC`)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	favorites := []string{}
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err == nil {
			favorites = append(favorites, username)
		}
	}

	return favorites, nil
}
