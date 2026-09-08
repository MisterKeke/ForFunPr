package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	favoriteSourceTelegram         = "telegram"
	favoriteSourceYouTube          = "youtube"
	favoriteCategoriesChangedEvent = "favorite-categories:changed"
)

type FavoriteCategory struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	Color        string `json:"color,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	DisplayOrder int    `json:"display_order"`
}

type FavoriteCategoryWriteRequest struct {
	Name         string `json:"name"`
	Source       string `json:"source"`
	Color        string `json:"color"`
	DisplayOrder *int   `json:"display_order,omitempty"`
}

type FavoriteCategoryMutationResult struct {
	Category          FavoriteCategory `json:"category"`
	Changed           bool             `json:"changed"`
	AffectedFavorites int              `json:"affected_favorites"`
}

type FavoriteCategoryDeleteRequest struct {
	ID               int    `json:"id"`
	Mode             string `json:"mode"`
	TargetCategoryID *int   `json:"target_category_id,omitempty"`
}

type FavoriteCategoryDeleteResult struct {
	DeletedID         int    `json:"deleted_id"`
	Source            string `json:"source"`
	Changed           bool   `json:"changed"`
	AffectedFavorites int    `json:"affected_favorites"`
}

type FavoriteCategoryReorderRequest struct {
	Source      string `json:"source"`
	CategoryIDs []int  `json:"category_ids"`
}

var favoriteCategoryColorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func (a *Service) EmitFavoriteCategoriesChanged(source string) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, favoriteCategoriesChangedEvent, source)
	}
}

type FavoriteChannel struct {
	Username   string `json:"username,omitempty"`
	ChannelID  string `json:"channel_id,omitempty"`
	CategoryID *int   `json:"category_id,omitempty"`
}

func normalizeCategoryName(name string) string {
	return strings.TrimSpace(name)
}

func normalizeFavoriteSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case favoriteSourceTelegram, favoriteSourceYouTube:
		return source
	default:
		return ""
	}
}

func NormalizeFavoriteSource(source string) (string, error) {
	normalized := normalizeFavoriteSource(source)
	if normalized == "" {
		return "", &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	return normalized, nil
}

func normalizedCategoryKey(source, name string) string {
	return normalizeFavoriteSource(source) + ":" + strings.ToLower(normalizeCategoryName(name))
}

func (a *Service) ListFavoriteCategories(source string) ([]FavoriteCategory, error) {
	return a.ListFavoriteCategoriesContext(a.requestContext(), source)
}

func (a *Service) ListFavoriteCategoriesContext(ctx context.Context, source string) ([]FavoriteCategory, error) {
	source = normalizeFavoriteSource(source)
	if source == "" {
		return nil, &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, name, source, COALESCE(color, ''), display_order,
		       created_at, COALESCE(updated_at, created_at)
		FROM favorite_categories
		WHERE source = ?
		ORDER BY display_order ASC, LOWER(name) ASC, id ASC
	`, source)
	if err != nil {
		return []FavoriteCategory{}, err
	}
	defer rows.Close()

	categories := []FavoriteCategory{}
	for rows.Next() {
		var category FavoriteCategory
		if err := rows.Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt); err != nil {
			return []FavoriteCategory{}, err
		}
		categories = append(categories, category)
	}

	return categories, rows.Err()
}

func (a *Service) CreateFavoriteCategory(name string, source string) (FavoriteCategory, error) {
	result, err := a.CreateFavoriteCategoryContext(a.requestContext(), FavoriteCategoryWriteRequest{Name: name, Source: source})
	return result.Category, err
}

func (a *Service) CreateFavoriteCategoryWithStatus(name string, source string) (FavoriteCategory, bool, error) {
	result, err := a.CreateFavoriteCategoryContext(a.requestContext(), FavoriteCategoryWriteRequest{Name: name, Source: source})
	return result.Category, result.Changed, err
}

func (a *Service) CreateFavoriteCategoryContext(ctx context.Context, request FavoriteCategoryWriteRequest) (FavoriteCategoryMutationResult, error) {
	name := normalizeCategoryName(request.Name)
	if name == "" {
		return FavoriteCategoryMutationResult{}, &ValidationError{Field: "name", Message: "category name cannot be empty"}
	}
	if len([]rune(name)) > 80 {
		return FavoriteCategoryMutationResult{}, &ValidationError{Field: "name", Message: "category name cannot exceed 80 characters"}
	}
	source := normalizeFavoriteSource(request.Source)
	if source == "" {
		return FavoriteCategoryMutationResult{}, &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	color, err := normalizeFavoriteCategoryColor(request.Color)
	if err != nil {
		return FavoriteCategoryMutationResult{}, err
	}
	displayOrder := 0
	if request.DisplayOrder != nil {
		if *request.DisplayOrder < 0 || *request.DisplayOrder > 1000000 {
			return FavoriteCategoryMutationResult{}, &ValidationError{Field: "display_order", Message: "display order must be between 0 and 1000000"}
		}
		displayOrder = *request.DisplayOrder
	} else if err := a.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(display_order), -1) + 1 FROM favorite_categories WHERE source = ?`, source).Scan(&displayOrder); err != nil {
		return FavoriteCategoryMutationResult{}, fmt.Errorf("choose favorite category order: %w", err)
	}
	nameNormalized := normalizedCategoryKey(source, name)

	var category FavoriteCategory
	err = a.db.QueryRowContext(ctx, `
		INSERT INTO favorite_categories (name, name_normalized, source, color, display_order, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name_normalized) DO NOTHING
		RETURNING id, name, source, COALESCE(color, ''), display_order, created_at, COALESCE(updated_at, created_at)
	`, name, nameNormalized, source, color, displayOrder).Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		err = a.db.QueryRowContext(ctx, `
			SELECT id, name, source, COALESCE(color, ''), display_order, created_at, COALESCE(updated_at, created_at)
			FROM favorite_categories
			WHERE name_normalized = ?
		`, nameNormalized).Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt)
		if err != nil {
			return FavoriteCategoryMutationResult{}, fmt.Errorf("load existing favorite category: %w", err)
		}
		return FavoriteCategoryMutationResult{Category: category}, nil
	}
	if err != nil {
		return FavoriteCategoryMutationResult{}, fmt.Errorf("create favorite category: %w", err)
	}

	return FavoriteCategoryMutationResult{Category: category, Changed: true}, nil
}

func (a *Service) RenameFavoriteCategory(id int, name string) (FavoriteCategory, error) {
	current, err := a.getFavoriteCategoryContext(a.requestContext(), id)
	if err != nil {
		return FavoriteCategory{}, err
	}
	result, err := a.UpdateFavoriteCategoryContext(a.requestContext(), id, FavoriteCategoryWriteRequest{
		Name: name, Source: current.Source, Color: current.Color,
	})
	return result.Category, err
}

func (a *Service) UpdateFavoriteCategoryContext(ctx context.Context, id int, request FavoriteCategoryWriteRequest) (FavoriteCategoryMutationResult, error) {
	if id <= 0 {
		return FavoriteCategoryMutationResult{}, &ValidationError{
			Field: "id", Message: "category ID must be a positive integer",
		}
	}
	name := normalizeCategoryName(request.Name)
	if name == "" {
		return FavoriteCategoryMutationResult{}, &ValidationError{
			Field: "name", Message: "category name cannot be empty",
		}
	}
	if len([]rune(name)) > 80 {
		return FavoriteCategoryMutationResult{}, &ValidationError{Field: "name", Message: "category name cannot exceed 80 characters"}
	}
	color, err := normalizeFavoriteCategoryColor(request.Color)
	if err != nil {
		return FavoriteCategoryMutationResult{}, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return FavoriteCategoryMutationResult{}, fmt.Errorf("begin favorite category update: %w", err)
	}
	defer tx.Rollback()

	var current FavoriteCategory
	if err := tx.QueryRowContext(ctx,
		`SELECT id, name, source, COALESCE(color, ''), display_order, created_at, COALESCE(updated_at, created_at)
		 FROM favorite_categories WHERE id = ?`,
		id,
	).Scan(&current.ID, &current.Name, &current.Source, &current.Color, &current.DisplayOrder, &current.CreatedAt, &current.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FavoriteCategoryMutationResult{}, &NotFoundError{
				Resource: "favorite category", Key: fmt.Sprint(id),
			}
		}
		return FavoriteCategoryMutationResult{}, fmt.Errorf("load favorite category for update: %w", err)
	}
	if request.Source != "" && normalizeFavoriteSource(request.Source) != current.Source {
		return FavoriteCategoryMutationResult{}, &ValidationError{Field: "source", Message: "category source cannot be changed"}
	}
	displayOrder := current.DisplayOrder
	if request.DisplayOrder != nil {
		if *request.DisplayOrder < 0 || *request.DisplayOrder > 1000000 {
			return FavoriteCategoryMutationResult{}, &ValidationError{Field: "display_order", Message: "display order must be between 0 and 1000000"}
		}
		displayOrder = *request.DisplayOrder
	}
	if current.Name == name && current.Color == color && current.DisplayOrder == displayOrder {
		return FavoriteCategoryMutationResult{Category: current}, nil
	}

	nameNormalized := normalizedCategoryKey(current.Source, name)
	var conflictingID int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM favorite_categories
		WHERE name_normalized = ? AND id <> ?
	`, nameNormalized, id).Scan(&conflictingID)
	if err == nil {
		return FavoriteCategoryMutationResult{}, &ConflictError{
			Resource: "favorite category",
			Message:  "a favorite category with that name already exists for this source",
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FavoriteCategoryMutationResult{}, fmt.Errorf("check favorite category update conflict: %w", err)
	}

	var category FavoriteCategory
	err = tx.QueryRowContext(ctx, `
		UPDATE favorite_categories
		SET name = ?, name_normalized = ?, color = NULLIF(?, ''), display_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		RETURNING id, name, source, COALESCE(color, ''), display_order, created_at, COALESCE(updated_at, created_at)
	`, name, nameNormalized, color, displayOrder, id).Scan(
		&category.ID,
		&category.Name,
		&category.Source,
		&category.Color,
		&category.DisplayOrder,
		&category.CreatedAt,
		&category.UpdatedAt,
	)
	if err != nil {
		return FavoriteCategoryMutationResult{}, fmt.Errorf("update favorite category: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FavoriteCategoryMutationResult{}, fmt.Errorf("commit favorite category update: %w", err)
	}
	return FavoriteCategoryMutationResult{Category: category, Changed: true}, nil
}

func (a *Service) getFavoriteCategoryContext(ctx context.Context, id int) (FavoriteCategory, error) {
	var category FavoriteCategory
	err := a.db.QueryRowContext(ctx, `
		SELECT id, name, source, COALESCE(color, ''), display_order,
		       created_at, COALESCE(updated_at, created_at)
		FROM favorite_categories WHERE id = ?
	`, id).Scan(&category.ID, &category.Name, &category.Source, &category.Color,
		&category.DisplayOrder, &category.CreatedAt, &category.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FavoriteCategory{}, &NotFoundError{Resource: "favorite category", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return FavoriteCategory{}, fmt.Errorf("load favorite category: %w", err)
	}
	return category, nil
}

func (a *Service) GetFavoriteCategoryContext(ctx context.Context, id int) (FavoriteCategory, error) {
	if id <= 0 {
		return FavoriteCategory{}, &ValidationError{Field: "id", Message: "category ID must be positive"}
	}
	return a.getFavoriteCategoryContext(ctx, id)
}

func normalizeFavoriteCategoryColor(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if len(value) == 4 && strings.HasPrefix(value, "#") {
		value = fmt.Sprintf("#%c%c%c%c%c%c", value[1], value[1], value[2], value[2], value[3], value[3])
	}
	if !favoriteCategoryColorPattern.MatchString(value) {
		return "", &ValidationError{Field: "color", Message: "color must be a three- or six-digit hexadecimal color"}
	}
	return value, nil
}

func (a *Service) DeleteFavoriteCategoryContext(ctx context.Context, request FavoriteCategoryDeleteRequest) (FavoriteCategoryDeleteResult, error) {
	if request.ID <= 0 {
		return FavoriteCategoryDeleteResult{}, &ValidationError{Field: "id", Message: "category ID must be positive"}
	}
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if mode == "" {
		mode = "unassign"
	}
	if mode != "unassign" && mode != "move" {
		return FavoriteCategoryDeleteResult{}, &ValidationError{Field: "mode", Message: "delete mode must be unassign or move"}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return FavoriteCategoryDeleteResult{}, fmt.Errorf("begin favorite category deletion: %w", err)
	}
	defer tx.Rollback()
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT source FROM favorite_categories WHERE id = ?`, request.ID).Scan(&source); errors.Is(err, sql.ErrNoRows) {
		return FavoriteCategoryDeleteResult{}, &NotFoundError{Resource: "favorite category", Key: fmt.Sprint(request.ID)}
	} else if err != nil {
		return FavoriteCategoryDeleteResult{}, fmt.Errorf("load favorite category for deletion: %w", err)
	}
	table := "telegram_favorites"
	if source == favoriteSourceYouTube {
		table = "youtube_favorites"
	}
	var affected int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE category_id = ?", request.ID).Scan(&affected); err != nil {
		return FavoriteCategoryDeleteResult{}, fmt.Errorf("count affected favorites: %w", err)
	}
	var target any
	if mode == "move" {
		if request.TargetCategoryID == nil || *request.TargetCategoryID <= 0 || *request.TargetCategoryID == request.ID {
			return FavoriteCategoryDeleteResult{}, &ValidationError{Field: "target_category_id", Message: "move mode requires a different target category"}
		}
		var targetSource string
		if err := tx.QueryRowContext(ctx, `SELECT source FROM favorite_categories WHERE id = ?`, *request.TargetCategoryID).Scan(&targetSource); errors.Is(err, sql.ErrNoRows) {
			return FavoriteCategoryDeleteResult{}, &NotFoundError{Resource: "target favorite category", Key: fmt.Sprint(*request.TargetCategoryID)}
		} else if err != nil {
			return FavoriteCategoryDeleteResult{}, fmt.Errorf("load target favorite category: %w", err)
		}
		if targetSource != source {
			return FavoriteCategoryDeleteResult{}, &ValidationError{Field: "target_category_id", Message: "target category must belong to the same source"}
		}
		target = *request.TargetCategoryID
	}
	if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET category_id = ? WHERE category_id = ?", target, request.ID); err != nil {
		return FavoriteCategoryDeleteResult{}, fmt.Errorf("move affected favorites: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM favorite_categories WHERE id = ?`, request.ID); err != nil {
		return FavoriteCategoryDeleteResult{}, fmt.Errorf("delete favorite category: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FavoriteCategoryDeleteResult{}, fmt.Errorf("commit favorite category deletion: %w", err)
	}
	return FavoriteCategoryDeleteResult{DeletedID: request.ID, Source: source, Changed: true, AffectedFavorites: affected}, nil
}

func (a *Service) ReorderFavoriteCategoriesContext(ctx context.Context, request FavoriteCategoryReorderRequest) ([]FavoriteCategory, error) {
	source, err := NormalizeFavoriteSource(request.Source)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]struct{}, len(request.CategoryIDs))
	for _, id := range request.CategoryIDs {
		if id <= 0 {
			return nil, &ValidationError{Field: "category_ids", Message: "category IDs must be positive"}
		}
		if _, exists := seen[id]; exists {
			return nil, &ValidationError{Field: "category_ids", Message: "category IDs cannot be repeated"}
		}
		seen[id] = struct{}{}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin favorite category reorder: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM favorite_categories WHERE source = ?`, source)
	if err != nil {
		return nil, fmt.Errorf("list favorite categories for reorder: %w", err)
	}
	stored := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan favorite category for reorder: %w", err)
		}
		stored[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate favorite categories for reorder: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close favorite categories for reorder: %w", err)
	}
	if len(stored) != len(request.CategoryIDs) {
		return nil, &ValidationError{Field: "category_ids", Message: "category IDs must include every category for the selected source exactly once"}
	}
	for id := range seen {
		if _, exists := stored[id]; !exists {
			return nil, &ValidationError{Field: "category_ids", Message: "every category must belong to the selected source"}
		}
	}
	for position, id := range request.CategoryIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE favorite_categories SET display_order = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND source = ?
		`, position, id, source)
		if err != nil {
			return nil, fmt.Errorf("reorder favorite category: %w", err)
		}
		if err := requireSingleMutation(result, "reorder favorite category", "favorite category", false); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit favorite category reorder: %w", err)
	}
	items, err := a.ListFavoriteCategoriesContext(ctx, source)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].DisplayOrder < items[j].DisplayOrder })
	return items, nil
}

func ensureFavoriteCategoryExists(ctx context.Context, store appStateStore, categoryID int, source string) error {
	source = normalizeFavoriteSource(source)
	if source == "" {
		return &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	var id int
	err := store.QueryRowContext(ctx, `
		SELECT id
		FROM favorite_categories
		WHERE id = ? AND source = ?
	`, categoryID, source).Scan(&id)

	if err == sql.ErrNoRows {
		return &NotFoundError{Resource: "favorite category", Key: fmt.Sprint(categoryID)}
	}
	if err != nil {
		return fmt.Errorf("find favorite category: %w", err)
	}

	return nil
}
