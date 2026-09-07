package tools

import (
	"errors"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOptionalFlags(t *testing.T) {
	base := []string{"command"}
	if got := OptionalStringFlag(base, "--name", ""); !reflect.DeepEqual(got, base) {
		t.Fatalf("empty string flag = %#v", got)
	}
	if got := OptionalStringFlag(base, "--name", "value"); !reflect.DeepEqual(got, []string{"command", "--name", "value"}) {
		t.Fatalf("string flag = %#v", got)
	}

	zero := 0
	if got := OptionalIntFlag(base, "--before", nil); !reflect.DeepEqual(got, base) {
		t.Fatalf("nil integer flag = %#v", got)
	}
	if got := OptionalIntFlag(base, "--before", &zero); !reflect.DeepEqual(got, []string{"command", "--before", "0"}) {
		t.Fatalf("zero integer flag = %#v", got)
	}
}

func TestAddToolRequiresExplicitInputSchema(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("AddTool accepted a tool without an input schema")
		}
	}()
	AddTool[struct{}, struct{}](nil, &mcp.Tool{Name: "missing_schema"}, nil)
}

func TestResponse(t *testing.T) {
	result, output, err := Response("Loaded values.", []int{1, 2}, nil)
	if err != nil {
		t.Fatalf("Response returned %v", err)
	}
	if !reflect.DeepEqual(output, []int{1, 2}) {
		t.Fatalf("output = %#v", output)
	}
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("result content = %#v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "Loaded values." {
		t.Fatalf("summary content = %#v", result.Content[0])
	}

	wantErr := errors.New("failed")
	result, output, err = Response("ignored", []int{1}, wantErr)
	if !errors.Is(err, wantErr) || result != nil || output != nil {
		t.Fatalf("error response = (%#v, %#v, %v)", result, output, err)
	}
}

func TestToolAnnotations(t *testing.T) {
	read := ReadAnnotations(true)
	if !read.ReadOnlyHint || read.DestructiveHint == nil || *read.DestructiveHint ||
		read.OpenWorldHint == nil || !*read.OpenWorldHint {
		t.Fatalf("read annotations = %#v", read)
	}

	write := WriteAnnotations(true, true, false)
	if write.ReadOnlyHint || write.DestructiveHint == nil || !*write.DestructiveHint ||
		!write.IdempotentHint || write.OpenWorldHint == nil || *write.OpenWorldHint {
		t.Fatalf("write annotations = %#v", write)
	}
}
