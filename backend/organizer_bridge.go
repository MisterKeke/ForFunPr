package backend

func (a *App) ListNotes(filter NoteListFilter) (NoteListResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return NoteListResult{}, err
	}
	defer done()
	return service.ListNotesContext(ctx, filter)
}

func (a *App) GetNote(id int) (Note, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Note{}, err
	}
	defer done()
	return service.GetNoteContext(ctx, id)
}

func (a *App) CreateNote(request NoteCreateRequest) (Note, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Note{}, err
	}
	defer done()
	return service.CreateNoteContext(ctx, request)
}

func (a *App) UpdateNote(request NoteUpdateRequest) (Note, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Note{}, err
	}
	defer done()
	return service.UpdateNoteContext(ctx, request)
}

func (a *App) SetNotePinned(request NoteStateRequest) (Note, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Note{}, err
	}
	defer done()
	return service.SetNotePinnedContext(ctx, request)
}

func (a *App) SetNoteArchived(request NoteStateRequest) (Note, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Note{}, err
	}
	defer done()
	return service.SetNoteArchivedContext(ctx, request)
}

func (a *App) DeleteNote(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteNoteContext(ctx, id)
}

func (a *App) ListBookmarks(filter BookmarkFilter) (BookmarkListResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return BookmarkListResult{}, err
	}
	defer done()
	return service.ListBookmarksContext(ctx, filter)
}

func (a *App) GetBookmark(id int) (Bookmark, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Bookmark{}, err
	}
	defer done()
	return service.GetBookmarkContext(ctx, id)
}

func (a *App) CreateBookmark(request BookmarkCreateRequest) (Bookmark, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Bookmark{}, err
	}
	defer done()
	return service.CreateBookmarkContext(ctx, request)
}

func (a *App) UpdateBookmark(request BookmarkUpdateRequest) (Bookmark, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Bookmark{}, err
	}
	defer done()
	return service.UpdateBookmarkContext(ctx, request)
}

func (a *App) SetBookmarkRead(request BookmarkReadRequest) (Bookmark, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Bookmark{}, err
	}
	defer done()
	return service.SetBookmarkReadContext(ctx, request)
}

func (a *App) DeleteBookmark(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteBookmarkContext(ctx, id)
}

func (a *App) ListBookmarkTags() ([]string, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListBookmarkTagsContext(ctx)
}
