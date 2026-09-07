package cmd

import (
	"reflect"
	"sort"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandRegistersEveryPublicCommandGroup(t *testing.T) {
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(root.Commands()))
	for _, command := range root.Commands() {
		got = append(got, command.Name())
	}
	want := []string{
		"bookmarks", "currencies", "desktop-apps", "favorite-categories",
		"favorites", "health", "news", "notes", "posts", "setups",
		"steam-games", "tasks", "wallpapers", "weather",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("top-level commands = %v, want %v", got, want)
	}
	for _, flag := range []string{apiURLConfigKey, outputConfigKey} {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Fatalf("missing persistent flag --%s", flag)
		}
	}
}

func TestEveryCLILeafIsExecutable(t *testing.T) {
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		children := command.Commands()
		if len(children) == 0 {
			if command.Run == nil && command.RunE == nil {
				t.Errorf("leaf command %q has no execution function", command.CommandPath())
			}
			return
		}
		for _, child := range children {
			visit(child)
		}
	}
	for _, command := range root.Commands() {
		visit(command)
	}
}

func TestNativeExecutionCommandsExposeExplicitConfirmation(t *testing.T) {
	root, err := newRootCommand()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range [][]string{{"desktop-apps", "launch"}, {"setups", "start"}} {
		command, remaining, err := root.Find(path)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("find %v = %#v, %v", path, remaining, err)
		}
		if command.Flags().Lookup("confirm") == nil {
			t.Fatalf("%q has no --confirm flag", command.CommandPath())
		}
	}
	if err := requireExecutionConfirmation(false); err == nil {
		t.Fatal("native execution was allowed without confirmation")
	}
	if err := requireExecutionConfirmation(true); err != nil {
		t.Fatalf("confirmed native execution = %v", err)
	}
}

func TestOrganizerCLIParsingHelpers(t *testing.T) {
	if got, err := parsePositiveCLIInteger("id", " 42 "); err != nil || got != 42 {
		t.Fatalf("parsePositiveCLIInteger = %d, %v", got, err)
	}
	for _, value := range []string{"", "0", "-1", "word"} {
		if _, err := parsePositiveCLIInteger("id", value); err == nil {
			t.Fatalf("parsePositiveCLIInteger accepted %q", value)
		}
	}
	if got := splitCommaValues(" one, two, two, three "); !reflect.DeepEqual(got, []string{"one", "two", "two", "three"}) {
		t.Fatalf("splitCommaValues = %#v", got)
	}
}
