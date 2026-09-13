package launcher

import (
	"context"
	"testing"
)

func TestRepositoryOpenerRejectsEmptyArguments(t *testing.T) {
	opener := NewRepositoryOpener()
	if err := opener.OpenFolder(context.Background(), ""); err == nil {
		t.Fatal("empty repository folder was accepted")
	}
	if err := opener.OpenEditor(context.Background(), "", "C:\\Repositories\\Something"); err == nil {
		t.Fatal("empty editor executable was accepted")
	}
	if err := opener.OpenEditor(context.Background(), "C:\\Editors\\Editor.exe", ""); err == nil {
		t.Fatal("empty repository path was accepted")
	}
}
