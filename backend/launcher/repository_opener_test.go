package launcher

import "testing"

func TestRepositoryOpenerRejectsEmptyArguments(t *testing.T) {
	opener := NewRepositoryOpener()
	if err := opener.OpenFolder(""); err == nil {
		t.Fatal("empty repository folder was accepted")
	}
	if err := opener.OpenEditor("", "C:\\Repositories\\Something"); err == nil {
		t.Fatal("empty editor executable was accepted")
	}
	if err := opener.OpenEditor("C:\\Editors\\Editor.exe", ""); err == nil {
		t.Fatal("empty repository path was accepted")
	}
}
