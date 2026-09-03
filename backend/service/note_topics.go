package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const (
	maximumNoteTopicTitleLength = 120
	maximumNoteTopicCoordinate  = 100000
)

type NoteTopic struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	BlockCount int    `json:"block_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type NoteTopicBlock struct {
	ID                 int     `json:"id"`
	TopicID            int     `json:"topic_id"`
	NoteID             int     `json:"note_id"`
	PositionX          float64 `json:"position_x"`
	PositionY          float64 `json:"position_y"`
	Title              string  `json:"title"`
	Preview            string  `json:"preview"`
	Pinned             bool    `json:"pinned"`
	Archived           bool    `json:"archived"`
	UpdatedAt          string  `json:"updated_at"`
	LinkedTaskCount    int     `json:"linked_task_count"`
	CompletedTaskCount int     `json:"completed_task_count"`
}

type NoteTopicConnection struct {
	ID           int    `json:"id"`
	TopicID      int    `json:"topic_id"`
	FromBlockID  int    `json:"from_block_id"`
	ToBlockID    int    `json:"to_block_id"`
	RelationType string `json:"relation_type"`
	CreatedAt    string `json:"created_at"`
}

type NoteTopicBoard struct {
	Topic       NoteTopic             `json:"topic"`
	Blocks      []NoteTopicBlock      `json:"blocks"`
	Connections []NoteTopicConnection `json:"connections"`
}

type NoteTopicWriteRequest struct {
	ID    int    `json:"id,omitempty"`
	Title string `json:"title"`
}

type NoteTopicBlockCreateRequest struct {
	TopicID   int     `json:"topic_id"`
	NoteID    int     `json:"note_id"`
	PositionX float64 `json:"position_x"`
	PositionY float64 `json:"position_y"`
}

type NoteTopicBlockPositionRequest struct {
	BlockID   int     `json:"block_id"`
	PositionX float64 `json:"position_x"`
	PositionY float64 `json:"position_y"`
}

type NoteTopicConnectionCreateRequest struct {
	TopicID      int    `json:"topic_id"`
	FromBlockID  int    `json:"from_block_id"`
	ToBlockID    int    `json:"to_block_id"`
	RelationType string `json:"relation_type"`
}

type NoteTodoConnectionRequest struct {
	NoteID int `json:"note_id"`
	TodoID int `json:"todo_id"`
}

func (a *Service) ListNoteTopicsContext(ctx context.Context) ([]NoteTopic, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT topics.id, topics.title, COUNT(blocks.id),
		       topics.created_at, topics.updated_at
		FROM note_topics AS topics
		LEFT JOIN note_topic_blocks AS blocks ON blocks.topic_id = topics.id
		GROUP BY topics.id
		ORDER BY topics.updated_at DESC, topics.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list note topics: %w", err)
	}
	defer rows.Close()

	topics := []NoteTopic{}
	for rows.Next() {
		var topic NoteTopic
		if err := rows.Scan(
			&topic.ID,
			&topic.Title,
			&topic.BlockCount,
			&topic.CreatedAt,
			&topic.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan note topic: %w", err)
		}
		topics = append(topics, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate note topics: %w", err)
	}
	return topics, nil
}

func (a *Service) GetNoteTopicBoardContext(ctx context.Context, topicID int) (NoteTopicBoard, error) {
	if err := validatePositiveRelationshipID("topic", topicID); err != nil {
		return NoteTopicBoard{}, err
	}

	topic, err := loadNoteTopicContext(ctx, a.db, topicID)
	if err != nil {
		return NoteTopicBoard{}, err
	}

	blockRows, err := a.db.QueryContext(ctx, `
		SELECT blocks.id, blocks.topic_id, blocks.note_id,
		       blocks.position_x, blocks.position_y,
		       notes.title, substr(notes.body, 1, 240),
		       notes.is_pinned, notes.is_archived, notes.updated_at,
		       COUNT(note_tasks.todo_id),
		       COALESCE(SUM(CASE WHEN todos.is_completed <> 0 THEN 1 ELSE 0 END), 0)
		FROM note_topic_blocks AS blocks
		JOIN notes ON notes.id = blocks.note_id
		LEFT JOIN note_todo_connections AS note_tasks ON note_tasks.note_id = notes.id
		LEFT JOIN todos ON todos.id = note_tasks.todo_id
		WHERE blocks.topic_id = ?
		GROUP BY blocks.id
		ORDER BY blocks.id
	`, topicID)
	if err != nil {
		return NoteTopicBoard{}, fmt.Errorf("list note topic blocks: %w", err)
	}

	blocks := []NoteTopicBlock{}
	for blockRows.Next() {
		var block NoteTopicBlock
		var preview string
		var pinned int
		var archived int
		if err := blockRows.Scan(
			&block.ID,
			&block.TopicID,
			&block.NoteID,
			&block.PositionX,
			&block.PositionY,
			&block.Title,
			&preview,
			&pinned,
			&archived,
			&block.UpdatedAt,
			&block.LinkedTaskCount,
			&block.CompletedTaskCount,
		); err != nil {
			blockRows.Close()
			return NoteTopicBoard{}, fmt.Errorf("scan note topic block: %w", err)
		}
		block.Preview = notePreview(preview)
		block.Pinned = pinned != 0
		block.Archived = archived != 0
		blocks = append(blocks, block)
	}
	if err := blockRows.Err(); err != nil {
		blockRows.Close()
		return NoteTopicBoard{}, fmt.Errorf("iterate note topic blocks: %w", err)
	}
	if err := blockRows.Close(); err != nil {
		return NoteTopicBoard{}, fmt.Errorf("close note topic blocks: %w", err)
	}

	connectionRows, err := a.db.QueryContext(ctx, `
		SELECT id, topic_id, from_block_id, to_block_id, relation_type, created_at
		FROM note_topic_connections
		WHERE topic_id = ?
		ORDER BY id
	`, topicID)
	if err != nil {
		return NoteTopicBoard{}, fmt.Errorf("list note topic connections: %w", err)
	}
	defer connectionRows.Close()

	connections := []NoteTopicConnection{}
	for connectionRows.Next() {
		var connection NoteTopicConnection
		if err := connectionRows.Scan(
			&connection.ID,
			&connection.TopicID,
			&connection.FromBlockID,
			&connection.ToBlockID,
			&connection.RelationType,
			&connection.CreatedAt,
		); err != nil {
			return NoteTopicBoard{}, fmt.Errorf("scan note topic connection: %w", err)
		}
		connections = append(connections, connection)
	}
	if err := connectionRows.Err(); err != nil {
		return NoteTopicBoard{}, fmt.Errorf("iterate note topic connections: %w", err)
	}

	topic.BlockCount = len(blocks)
	return NoteTopicBoard{Topic: topic, Blocks: blocks, Connections: connections}, nil
}

func (a *Service) CreateNoteTopicContext(ctx context.Context, request NoteTopicWriteRequest) (NoteTopic, error) {
	title, err := normalizeNoteTopicTitle(request.Title)
	if err != nil {
		return NoteTopic{}, err
	}
	result, err := a.db.ExecContext(ctx, `
		INSERT INTO note_topics (title) VALUES (?)
	`, title)
	if err != nil {
		return NoteTopic{}, fmt.Errorf("create note topic: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return NoteTopic{}, fmt.Errorf("read created note topic ID: %w", err)
	}
	return loadNoteTopicContext(ctx, a.db, int(id))
}

func (a *Service) RenameNoteTopicContext(ctx context.Context, request NoteTopicWriteRequest) (NoteTopic, error) {
	if err := validatePositiveRelationshipID("topic", request.ID); err != nil {
		return NoteTopic{}, err
	}
	title, err := normalizeNoteTopicTitle(request.Title)
	if err != nil {
		return NoteTopic{}, err
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE note_topics
		SET title = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, title, request.ID)
	if err != nil {
		return NoteTopic{}, fmt.Errorf("rename note topic: %w", err)
	}
	if err := requireRelationshipMutation(result, "rename note topic", "note topic", request.ID); err != nil {
		return NoteTopic{}, err
	}
	return loadNoteTopicContext(ctx, a.db, request.ID)
}

func (a *Service) DeleteNoteTopicContext(ctx context.Context, topicID int) error {
	if err := validatePositiveRelationshipID("topic", topicID); err != nil {
		return err
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM note_topics WHERE id = ?`, topicID)
	if err != nil {
		return fmt.Errorf("delete note topic: %w", err)
	}
	return requireRelationshipMutation(result, "delete note topic", "note topic", topicID)
}

func (a *Service) AddNoteTopicBlockContext(
	ctx context.Context,
	request NoteTopicBlockCreateRequest,
) (NoteTopicBlock, error) {
	if err := validatePositiveRelationshipID("topic", request.TopicID); err != nil {
		return NoteTopicBlock{}, err
	}
	if err := validateNoteID(request.NoteID); err != nil {
		return NoteTopicBlock{}, err
	}
	x, err := normalizeNoteTopicCoordinate("position_x", request.PositionX)
	if err != nil {
		return NoteTopicBlock{}, err
	}
	y, err := normalizeNoteTopicCoordinate("position_y", request.PositionY)
	if err != nil {
		return NoteTopicBlock{}, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return NoteTopicBlock{}, fmt.Errorf("begin add note topic block: %w", err)
	}
	defer tx.Rollback()
	if err := requireRelationshipResourceContext(ctx, tx, "note_topics", "note topic", request.TopicID); err != nil {
		return NoteTopicBlock{}, err
	}
	if err := requireRelationshipResourceContext(ctx, tx, "notes", "note", request.NoteID); err != nil {
		return NoteTopicBlock{}, err
	}
	var existingID int
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM note_topic_blocks WHERE topic_id = ? AND note_id = ?
	`, request.TopicID, request.NoteID).Scan(&existingID)
	if err == nil {
		return NoteTopicBlock{}, &ValidationError{Field: "note_id", Message: "note is already on this topic"}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return NoteTopicBlock{}, fmt.Errorf("check note topic block: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO note_topic_blocks (topic_id, note_id, position_x, position_y)
		VALUES (?, ?, ?, ?)
	`, request.TopicID, request.NoteID, x, y)
	if err != nil {
		return NoteTopicBlock{}, fmt.Errorf("add note topic block: %w", err)
	}
	blockID, err := result.LastInsertId()
	if err != nil {
		return NoteTopicBlock{}, fmt.Errorf("read created note topic block ID: %w", err)
	}
	if err := touchNoteTopicContext(ctx, tx, request.TopicID); err != nil {
		return NoteTopicBlock{}, err
	}
	if err := tx.Commit(); err != nil {
		return NoteTopicBlock{}, fmt.Errorf("commit add note topic block: %w", err)
	}
	return NoteTopicBlock{
		ID: int(blockID), TopicID: request.TopicID, NoteID: request.NoteID,
		PositionX: x, PositionY: y,
	}, nil
}

func (a *Service) UpdateNoteTopicBlockPositionContext(
	ctx context.Context,
	request NoteTopicBlockPositionRequest,
) error {
	if err := validatePositiveRelationshipID("block", request.BlockID); err != nil {
		return err
	}
	x, err := normalizeNoteTopicCoordinate("position_x", request.PositionX)
	if err != nil {
		return err
	}
	y, err := normalizeNoteTopicCoordinate("position_y", request.PositionY)
	if err != nil {
		return err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin move note topic block: %w", err)
	}
	defer tx.Rollback()
	var topicID int
	if err := tx.QueryRowContext(ctx, `
		SELECT topic_id FROM note_topic_blocks WHERE id = ?
	`, request.BlockID).Scan(&topicID); errors.Is(err, sql.ErrNoRows) {
		return &NotFoundError{Resource: "note topic block", Key: fmt.Sprint(request.BlockID)}
	} else if err != nil {
		return fmt.Errorf("load note topic block: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE note_topic_blocks SET position_x = ?, position_y = ? WHERE id = ?
	`, x, y, request.BlockID); err != nil {
		return fmt.Errorf("move note topic block: %w", err)
	}
	if err := touchNoteTopicContext(ctx, tx, topicID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit move note topic block: %w", err)
	}
	return nil
}

func (a *Service) DeleteNoteTopicBlockContext(ctx context.Context, blockID int) error {
	if err := validatePositiveRelationshipID("block", blockID); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete note topic block: %w", err)
	}
	defer tx.Rollback()
	var topicID int
	if err := tx.QueryRowContext(ctx, `
		SELECT topic_id FROM note_topic_blocks WHERE id = ?
	`, blockID).Scan(&topicID); errors.Is(err, sql.ErrNoRows) {
		return &NotFoundError{Resource: "note topic block", Key: fmt.Sprint(blockID)}
	} else if err != nil {
		return fmt.Errorf("load note topic block: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM note_topic_blocks WHERE id = ?`, blockID); err != nil {
		return fmt.Errorf("delete note topic block: %w", err)
	}
	if err := touchNoteTopicContext(ctx, tx, topicID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete note topic block: %w", err)
	}
	return nil
}

func (a *Service) CreateNoteTopicConnectionContext(
	ctx context.Context,
	request NoteTopicConnectionCreateRequest,
) (NoteTopicConnection, error) {
	if err := validatePositiveRelationshipID("topic", request.TopicID); err != nil {
		return NoteTopicConnection{}, err
	}
	if err := validatePositiveRelationshipID("from block", request.FromBlockID); err != nil {
		return NoteTopicConnection{}, err
	}
	if err := validatePositiveRelationshipID("to block", request.ToBlockID); err != nil {
		return NoteTopicConnection{}, err
	}
	if request.FromBlockID == request.ToBlockID {
		return NoteTopicConnection{}, &ValidationError{Field: "to_block_id", Message: "a block cannot connect to itself"}
	}
	relationType, err := normalizeNoteTopicRelationType(request.RelationType)
	if err != nil {
		return NoteTopicConnection{}, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return NoteTopicConnection{}, fmt.Errorf("begin create note topic connection: %w", err)
	}
	defer tx.Rollback()
	if err := requireRelationshipResourceContext(ctx, tx, "note_topics", "note topic", request.TopicID); err != nil {
		return NoteTopicConnection{}, err
	}
	for _, blockID := range []int{request.FromBlockID, request.ToBlockID} {
		var actualTopicID int
		if err := tx.QueryRowContext(ctx, `
			SELECT topic_id FROM note_topic_blocks WHERE id = ?
		`, blockID).Scan(&actualTopicID); errors.Is(err, sql.ErrNoRows) {
			return NoteTopicConnection{}, &NotFoundError{Resource: "note topic block", Key: fmt.Sprint(blockID)}
		} else if err != nil {
			return NoteTopicConnection{}, fmt.Errorf("load note topic block: %w", err)
		}
		if actualTopicID != request.TopicID {
			return NoteTopicConnection{}, &ValidationError{
				Field: "topic_id", Message: "both blocks must belong to the selected topic",
			}
		}
	}

	if relationType == "related" {
		var duplicate int
		err := tx.QueryRowContext(ctx, `
			SELECT id
			FROM note_topic_connections
			WHERE topic_id = ? AND relation_type = 'related'
			  AND ((from_block_id = ? AND to_block_id = ?)
			       OR (from_block_id = ? AND to_block_id = ?))
		`, request.TopicID, request.FromBlockID, request.ToBlockID,
			request.ToBlockID, request.FromBlockID).Scan(&duplicate)
		if err == nil {
			return NoteTopicConnection{}, &ValidationError{Field: "connection", Message: "these blocks are already related"}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return NoteTopicConnection{}, fmt.Errorf("check related note topic connection: %w", err)
		}
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO note_topic_connections (
			topic_id, from_block_id, to_block_id, relation_type
		) VALUES (?, ?, ?, ?)
	`, request.TopicID, request.FromBlockID, request.ToBlockID, relationType)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return NoteTopicConnection{}, &ValidationError{Field: "connection", Message: "this connection already exists"}
		}
		return NoteTopicConnection{}, fmt.Errorf("create note topic connection: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return NoteTopicConnection{}, fmt.Errorf("read note topic connection ID: %w", err)
	}
	if err := touchNoteTopicContext(ctx, tx, request.TopicID); err != nil {
		return NoteTopicConnection{}, err
	}
	if err := tx.Commit(); err != nil {
		return NoteTopicConnection{}, fmt.Errorf("commit note topic connection: %w", err)
	}
	return NoteTopicConnection{
		ID: int(id), TopicID: request.TopicID,
		FromBlockID: request.FromBlockID, ToBlockID: request.ToBlockID,
		RelationType: relationType,
	}, nil
}

func (a *Service) DeleteNoteTopicConnectionContext(ctx context.Context, connectionID int) error {
	if err := validatePositiveRelationshipID("connection", connectionID); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete note topic connection: %w", err)
	}
	defer tx.Rollback()
	var topicID int
	if err := tx.QueryRowContext(ctx, `
		SELECT topic_id FROM note_topic_connections WHERE id = ?
	`, connectionID).Scan(&topicID); errors.Is(err, sql.ErrNoRows) {
		return &NotFoundError{Resource: "note topic connection", Key: fmt.Sprint(connectionID)}
	} else if err != nil {
		return fmt.Errorf("load note topic connection: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM note_topic_connections WHERE id = ?
	`, connectionID); err != nil {
		return fmt.Errorf("delete note topic connection: %w", err)
	}
	if err := touchNoteTopicContext(ctx, tx, topicID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete note topic connection: %w", err)
	}
	return nil
}

func (a *Service) ListNoteTodosContext(ctx context.Context, noteID int) ([]Todo, error) {
	if err := validateNoteID(noteID); err != nil {
		return nil, err
	}
	if err := requireRelationshipResourceContext(ctx, a.db, "notes", "note", noteID); err != nil {
		return nil, err
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT todos.id, todos.title, todos.description, todos.is_completed,
		       todos.created_at, todos.due_date, todos.priority, todos.difficulty
		FROM note_todo_connections AS connections
		JOIN todos ON todos.id = connections.todo_id
		WHERE connections.note_id = ?
		ORDER BY todos.is_completed, connections.created_at DESC, todos.id DESC
	`, noteID)
	if err != nil {
		return nil, fmt.Errorf("list note tasks: %w", err)
	}
	todos, err := scanTodos(rows)
	if err != nil {
		return nil, err
	}
	return hydrateTodoRelations(ctx, a.db, todos)
}

func (a *Service) ListTodoNotesContext(ctx context.Context, todoID int) ([]NoteSummary, error) {
	if err := validateTodoID(todoID); err != nil {
		return nil, err
	}
	if err := requireRelationshipResourceContext(ctx, a.db, "todos", "todo", todoID); err != nil {
		return nil, err
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT notes.id, notes.title, substr(notes.body, 1, 240),
		       notes.is_pinned, notes.is_archived, notes.revision,
		       notes.created_at, notes.updated_at
		FROM note_todo_connections AS connections
		JOIN notes ON notes.id = connections.note_id
		WHERE connections.todo_id = ?
		ORDER BY notes.updated_at DESC, notes.id DESC
	`, todoID)
	if err != nil {
		return nil, fmt.Errorf("list task notes: %w", err)
	}
	defer rows.Close()
	notes := []NoteSummary{}
	for rows.Next() {
		var note NoteSummary
		var preview string
		var pinned int
		var archived int
		if err := rows.Scan(
			&note.ID,
			&note.Title,
			&preview,
			&pinned,
			&archived,
			&note.Revision,
			&note.CreatedAt,
			&note.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task note: %w", err)
		}
		note.Preview = notePreview(preview)
		note.Pinned = pinned != 0
		note.Archived = archived != 0
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task notes: %w", err)
	}
	return notes, nil
}

func (a *Service) LinkNoteTodoContext(ctx context.Context, request NoteTodoConnectionRequest) error {
	if err := validateNoteID(request.NoteID); err != nil {
		return err
	}
	if err := validateTodoID(request.TodoID); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin link note task: %w", err)
	}
	defer tx.Rollback()
	if err := requireRelationshipResourceContext(ctx, tx, "notes", "note", request.NoteID); err != nil {
		return err
	}
	if err := requireRelationshipResourceContext(ctx, tx, "todos", "todo", request.TodoID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO note_todo_connections (note_id, todo_id)
		VALUES (?, ?)
		ON CONFLICT(note_id, todo_id) DO NOTHING
	`, request.NoteID, request.TodoID); err != nil {
		return fmt.Errorf("link note task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit link note task: %w", err)
	}
	return nil
}

func (a *Service) UnlinkNoteTodoContext(ctx context.Context, request NoteTodoConnectionRequest) error {
	if err := validateNoteID(request.NoteID); err != nil {
		return err
	}
	if err := validateTodoID(request.TodoID); err != nil {
		return err
	}
	if _, err := a.db.ExecContext(ctx, `
		DELETE FROM note_todo_connections WHERE note_id = ? AND todo_id = ?
	`, request.NoteID, request.TodoID); err != nil {
		return fmt.Errorf("unlink note task: %w", err)
	}
	return nil
}

type relationshipQueryStore interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadNoteTopicContext(
	ctx context.Context,
	store relationshipQueryStore,
	topicID int,
) (NoteTopic, error) {
	var topic NoteTopic
	err := store.QueryRowContext(ctx, `
		SELECT topics.id, topics.title,
		       (SELECT COUNT(*) FROM note_topic_blocks WHERE topic_id = topics.id),
		       topics.created_at, topics.updated_at
		FROM note_topics AS topics
		WHERE topics.id = ?
	`, topicID).Scan(
		&topic.ID,
		&topic.Title,
		&topic.BlockCount,
		&topic.CreatedAt,
		&topic.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return NoteTopic{}, &NotFoundError{Resource: "note topic", Key: fmt.Sprint(topicID)}
	}
	if err != nil {
		return NoteTopic{}, fmt.Errorf("load note topic: %w", err)
	}
	return topic, nil
}

func normalizeNoteTopicTitle(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &ValidationError{Field: "title", Message: "topic title is required"}
	}
	if utf8.RuneCountInString(value) > maximumNoteTopicTitleLength {
		return "", &ValidationError{
			Field:   "title",
			Message: fmt.Sprintf("topic title cannot exceed %d characters", maximumNoteTopicTitleLength),
		}
	}
	return value, nil
}

func normalizeNoteTopicCoordinate(field string, value float64) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maximumNoteTopicCoordinate {
		return 0, &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be between 0 and %d", field, maximumNoteTopicCoordinate),
		}
	}
	return math.Round(value*10) / 10, nil
}

func normalizeNoteTopicRelationType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "leads_to", nil
	}
	if value != "leads_to" && value != "related" {
		return "", &ValidationError{
			Field: "relation_type", Message: "relation type must be leads_to or related",
		}
	}
	return value, nil
}

func validatePositiveRelationshipID(name string, id int) error {
	if id <= 0 {
		return &ValidationError{Field: name + "_id", Message: name + " ID must be a positive integer"}
	}
	return nil
}

func requireRelationshipResourceContext(
	ctx context.Context,
	store relationshipQueryStore,
	table string,
	resource string,
	id int,
) error {
	var exists int
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = ?)", table)
	if err := store.QueryRowContext(ctx, query, id).Scan(&exists); err != nil {
		return fmt.Errorf("check %s: %w", resource, err)
	}
	if exists == 0 {
		return &NotFoundError{Resource: resource, Key: fmt.Sprint(id)}
	}
	return nil
}

func requireRelationshipMutation(
	result sql.Result,
	operation string,
	resource string,
	id int,
) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check %s result: %w", operation, err)
	}
	if affected == 0 {
		return &NotFoundError{Resource: resource, Key: fmt.Sprint(id)}
	}
	if affected != 1 {
		return fmt.Errorf("%s affected %d rows", operation, affected)
	}
	return nil
}

func touchNoteTopicContext(ctx context.Context, tx *sql.Tx, topicID int) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE note_topics SET updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, topicID); err != nil {
		return fmt.Errorf("touch note topic: %w", err)
	}
	return nil
}
