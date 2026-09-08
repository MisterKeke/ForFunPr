package backend

func (a *App) ListNoteTopics() ([]NoteTopic, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListNoteTopicsContext(ctx)
}

func (a *App) ListNoteTopicsPage(filter NoteTopicListFilter) (NoteTopicListResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicListResult{}, err
	}
	defer done()
	return service.ListNoteTopicsPageContext(ctx, filter)
}

func (a *App) GetNoteTopicBoard(topicID int) (NoteTopicBoard, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicBoard{}, err
	}
	defer done()
	return service.GetNoteTopicBoardContext(ctx, topicID)
}

func (a *App) CreateNoteTopic(request NoteTopicWriteRequest) (NoteTopic, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopic{}, err
	}
	defer done()
	return service.CreateNoteTopicContext(ctx, request)
}

func (a *App) RenameNoteTopic(request NoteTopicWriteRequest) (NoteTopic, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopic{}, err
	}
	defer done()
	return service.RenameNoteTopicContext(ctx, request)
}

func (a *App) DeleteNoteTopic(topicID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteNoteTopicContext(ctx, topicID)
}

func (a *App) DeleteNoteTopicWithRevision(request NoteTopicIDRequest) (NoteTopicMutationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicMutationResult{}, err
	}
	defer done()
	return service.DeleteNoteTopicWithRevisionContext(ctx, request)
}

func (a *App) AddNoteTopicBlock(request NoteTopicBlockCreateRequest) (NoteTopicBlock, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicBlock{}, err
	}
	defer done()
	return service.AddNoteTopicBlockContext(ctx, request)
}

func (a *App) UpdateNoteTopicBlockPosition(request NoteTopicBlockPositionRequest) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.UpdateNoteTopicBlockPositionContext(ctx, request)
}

func (a *App) UpdateNoteTopicBlockPositions(request NoteTopicBlockPositionsRequest) (NoteTopicMutationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicMutationResult{}, err
	}
	defer done()
	return service.UpdateNoteTopicBlockPositionsContext(ctx, request)
}

func (a *App) SearchNoteTopicPicker(filter NoteTopicPickerFilter) (NoteTopicPickerResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicPickerResult{}, err
	}
	defer done()
	return service.SearchNoteTopicPickerContext(ctx, filter)
}

func (a *App) DeleteNoteTopicBlock(blockID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteNoteTopicBlockContext(ctx, blockID)
}

func (a *App) DeleteNoteTopicBlockWithRevision(request NoteTopicIDRequest) (NoteTopicMutationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicMutationResult{}, err
	}
	defer done()
	return service.DeleteNoteTopicBlockWithRevisionContext(ctx, request)
}

func (a *App) CreateNoteTopicConnection(
	request NoteTopicConnectionCreateRequest,
) (NoteTopicConnection, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicConnection{}, err
	}
	defer done()
	return service.CreateNoteTopicConnectionContext(ctx, request)
}

func (a *App) DeleteNoteTopicConnection(connectionID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteNoteTopicConnectionContext(ctx, connectionID)
}

func (a *App) DeleteNoteTopicConnectionWithRevision(request NoteTopicIDRequest) (NoteTopicMutationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTopicMutationResult{}, err
	}
	defer done()
	return service.DeleteNoteTopicConnectionWithRevisionContext(ctx, request)
}

func (a *App) ListNoteTodos(noteID int) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListNoteTodosContext(ctx, noteID)
}

func (a *App) ListTodoNotes(todoID int) ([]NoteSummary, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListTodoNotesContext(ctx, todoID)
}

func (a *App) LinkNoteTodo(request NoteTodoConnectionRequest) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.LinkNoteTodoContext(ctx, request)
}

func (a *App) LinkNoteTodoWithStatus(request NoteTodoConnectionRequest) (NoteTodoMutationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTodoMutationResult{}, err
	}
	defer done()
	return service.LinkNoteTodoWithStatusContext(ctx, request)
}

func (a *App) UnlinkNoteTodo(request NoteTodoConnectionRequest) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.UnlinkNoteTodoContext(ctx, request)
}

func (a *App) UnlinkNoteTodoWithStatus(request NoteTodoConnectionRequest) (NoteTodoMutationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteTodoMutationResult{}, err
	}
	defer done()
	return service.UnlinkNoteTodoWithStatusContext(ctx, request)
}
