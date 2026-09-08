package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Note struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type NoteSummary struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Preview   string `json:"preview"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type NoteListResult struct {
	Notes  []NoteSummary `json:"notes"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type NoteListFilter struct {
	Query   string
	Archive string
	Pinned  *bool
	Limit   int
	Offset  int
}

type NoteCreateRequest struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned"`
}

type NoteUpdateRequest struct {
	Title            string `json:"title"`
	Body             string `json:"body"`
	ExpectedRevision int    `json:"expected_revision"`
}

type NoteStateRequest struct {
	Value            bool `json:"value"`
	ExpectedRevision int  `json:"expected_revision"`
}

type NoteTopic struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	BlockCount int    `json:"block_count"`
	Revision   int    `json:"revision"`
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

type NoteTopicListResult struct {
	Items  []NoteTopic `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type NoteTopicPickerResult struct {
	Items  []NoteSummary `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type NoteTopicWriteRequest struct {
	Title            string `json:"title"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

type NoteTopicBlockCreateRequest struct {
	NoteID           int     `json:"note_id"`
	PositionX        float64 `json:"position_x"`
	PositionY        float64 `json:"position_y"`
	ExpectedRevision *int    `json:"expected_revision,omitempty"`
}

type NoteTopicBlockPosition struct {
	BlockID   int     `json:"block_id"`
	PositionX float64 `json:"position_x"`
	PositionY float64 `json:"position_y"`
}

type NoteTopicBlockPositionsRequest struct {
	Positions        []NoteTopicBlockPosition `json:"positions"`
	ExpectedRevision *int                     `json:"expected_revision,omitempty"`
}

type NoteTopicConnectionCreateRequest struct {
	FromBlockID      int    `json:"from_block_id"`
	ToBlockID        int    `json:"to_block_id"`
	RelationType     string `json:"relation_type"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

type NoteTopicMutationResult struct {
	Changed       bool `json:"changed"`
	TopicID       int  `json:"topic_id,omitempty"`
	TopicRevision int  `json:"topic_revision,omitempty"`
}

type NoteTodoMutationResult struct {
	Changed bool `json:"changed"`
	NoteID  int  `json:"note_id"`
	TodoID  int  `json:"todo_id"`
}

func (c *Client) ListNotes(ctx context.Context, filter NoteListFilter) (NoteListResult, error) {
	query := make(url.Values)
	if filter.Query != "" {
		query.Set("q", filter.Query)
	}
	if filter.Archive != "" {
		query.Set("archive", filter.Archive)
	}
	if filter.Pinned != nil {
		query.Set("pinned", strconv.FormatBool(*filter.Pinned))
	}
	if filter.Limit > 0 {
		query.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		query.Set("offset", strconv.Itoa(filter.Offset))
	}
	var result NoteListResult
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/notes", query, nil, &result)
	return result, err
}

func (c *Client) GetNote(ctx context.Context, id int) (Note, error) {
	var note Note
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/notes/%d", id), nil, nil, &note)
	return note, err
}

func (c *Client) CreateNote(ctx context.Context, request NoteCreateRequest) (Note, error) {
	var note Note
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/notes", nil, request, &note)
	return note, err
}

func (c *Client) UpdateNote(ctx context.Context, id int, request NoteUpdateRequest) (Note, error) {
	var note Note
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/notes/%d", id), nil, request, &note)
	return note, err
}

func (c *Client) SetNotePinned(ctx context.Context, id int, request NoteStateRequest) (Note, error) {
	var note Note
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/notes/%d/pinned", id), nil, request, &note)
	return note, err
}

func (c *Client) SetNoteArchived(ctx context.Context, id int, request NoteStateRequest) (Note, error) {
	var note Note
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/notes/%d/archived", id), nil, request, &note)
	return note, err
}

func (c *Client) DeleteNote(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/notes/%d", id), nil, nil, nil)
}

func (c *Client) ListNoteTopics(ctx context.Context, limit, offset int) (NoteTopicListResult, error) {
	query := make(url.Values)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	var result NoteTopicListResult
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/note-topics", query, nil, &result)
	return result, err
}

func (c *Client) GetNoteTopic(ctx context.Context, id int) (NoteTopic, error) {
	var result NoteTopic
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/note-topics/%d", id), nil, nil, &result)
	return result, err
}

func (c *Client) GetNoteTopicBoard(ctx context.Context, id int) (NoteTopicBoard, error) {
	var result NoteTopicBoard
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/note-topics/%d/board", id), nil, nil, &result)
	return result, err
}

func (c *Client) SearchNoteTopicPicker(ctx context.Context, id int, search string, limit, offset int) (NoteTopicPickerResult, error) {
	query := make(url.Values)
	if search != "" {
		query.Set("q", search)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	var result NoteTopicPickerResult
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/note-topics/%d/picker", id), query, nil, &result)
	return result, err
}

func (c *Client) CreateNoteTopic(ctx context.Context, request NoteTopicWriteRequest) (NoteTopic, error) {
	var result NoteTopic
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/note-topics", nil, request, &result)
	return result, err
}

func (c *Client) UpdateNoteTopic(ctx context.Context, id int, request NoteTopicWriteRequest) (NoteTopic, error) {
	var result NoteTopic
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/note-topics/%d", id), nil, request, &result)
	return result, err
}

func (c *Client) DeleteNoteTopic(ctx context.Context, id int, expectedRevision *int) (NoteTopicMutationResult, error) {
	var result NoteTopicMutationResult
	err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/note-topics/%d", id), revisionQuery(expectedRevision), nil, &result)
	return result, err
}

func (c *Client) AddNoteTopicBlock(ctx context.Context, topicID int, request NoteTopicBlockCreateRequest) (NoteTopicBlock, error) {
	var result NoteTopicBlock
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/note-topics/%d/blocks", topicID), nil, request, &result)
	return result, err
}

func (c *Client) UpdateNoteTopicBlockPositions(ctx context.Context, topicID int, request NoteTopicBlockPositionsRequest) (NoteTopicMutationResult, error) {
	var result NoteTopicMutationResult
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/note-topics/%d/blocks/positions", topicID), nil, request, &result)
	return result, err
}

func (c *Client) DeleteNoteTopicBlock(ctx context.Context, id int, expectedRevision *int) (NoteTopicMutationResult, error) {
	var result NoteTopicMutationResult
	err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/note-topic-blocks/%d", id), revisionQuery(expectedRevision), nil, &result)
	return result, err
}

func (c *Client) CreateNoteTopicConnection(ctx context.Context, topicID int, request NoteTopicConnectionCreateRequest) (NoteTopicConnection, error) {
	var result NoteTopicConnection
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/note-topics/%d/connections", topicID), nil, request, &result)
	return result, err
}

func (c *Client) DeleteNoteTopicConnection(ctx context.Context, id int, expectedRevision *int) (NoteTopicMutationResult, error) {
	var result NoteTopicMutationResult
	err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/note-topic-connections/%d", id), revisionQuery(expectedRevision), nil, &result)
	return result, err
}

func (c *Client) ListNoteTodos(ctx context.Context, noteID int) ([]Task, error) {
	var result []Task
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/notes/%d/tasks", noteID), nil, nil, &result)
	return result, err
}

func (c *Client) ListTodoNotes(ctx context.Context, todoID int) ([]NoteSummary, error) {
	var result []NoteSummary
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/notes", todoID), nil, nil, &result)
	return result, err
}

func (c *Client) LinkNoteTodo(ctx context.Context, noteID, todoID int) (NoteTodoMutationResult, error) {
	var result NoteTodoMutationResult
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/notes/%d/tasks", noteID), nil, map[string]int{"todo_id": todoID}, &result)
	return result, err
}

func (c *Client) UnlinkNoteTodo(ctx context.Context, noteID, todoID int) (NoteTodoMutationResult, error) {
	var result NoteTodoMutationResult
	err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/notes/%d/tasks/%d", noteID, todoID), nil, nil, &result)
	return result, err
}

func revisionQuery(expectedRevision *int) url.Values {
	if expectedRevision == nil {
		return nil
	}
	query := make(url.Values)
	query.Set("expected_revision", strconv.Itoa(*expectedRevision))
	return query
}
