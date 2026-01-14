package rexec

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func Test_envSlice(t *testing.T) {
	type args struct {
		env map[string]string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "nil",
			args: args{env: nil},
			want: []string{},
		},
		{
			name: "empty",
			args: args{env: map[string]string{}},
			want: []string{},
		},
		{
			name: "one",
			args: args{env: map[string]string{"key": "value"}},
			want: []string{"key=value"},
		},
		{
			name: "multiple",
			args: args{env: map[string]string{"key1": "value1", "key2": "value2"}},
			want: []string{"key1=value1", "key2=value2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envSlice(tt.args.env)
			want := tt.want

			// sort them for comparison
			// avoid Error: envSlice() = [key2=value2 key1=value1], want [key1=value1 key2=value2]
			sort.Strings(got)
			sort.Strings(want)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("envSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cmdSlice(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{s: ""},
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "one",
			args:    args{s: "ls"},
			want:    []string{"ls"},
			wantErr: false,
		},
		{
			name:    "multiple",
			args:    args{s: "ls -a /usr"},
			want:    []string{"ls", "-a", "/usr"},
			wantErr: false,
		},
		{
			name:    "quotedSimple",
			args:    args{s: "a b 'c d'"},
			want:    []string{"a", "b", "c d"},
			wantErr: false,
		},
		{
			name:    "quotedComplex",
			args:    args{s: "a b \"'c d' '\\\"e\\\" f'\""},
			want:    []string{"a", "b", "'c d' '\"e\" f'"},
			wantErr: false,
		},
		{
			name:    "quotedBad",
			args:    args{s: "a b 'c d"},
			want:    []string{"a", "b"},
			wantErr: true,
		},
		{
			name:    "quotedBad2",
			args:    args{s: "a b \"c d"},
			want:    []string{"a", "b"},
			wantErr: true,
		},
		{
			name:    "quotedRealWorld",
			args:    args{s: "cd /tmp && echo 'hello world'"},
			want:    []string{"cd", "/tmp", "&&", "echo", "hello world"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cmdSlice(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("cmdSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cmdSlice() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCommand_FromJson(t *testing.T) {
	jsonStr := []byte(`{
	"command": "env | grep REXEC",
	"workdir": "/tmp",
	"env": {
		"REXEC1": "VALUE1",
		"REXEC2": "VALUE2"
	}
}`)

	var cmd Command
	err := json.Unmarshal(jsonStr, &cmd)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	expectedCmd := Command{
		Command: "env | grep REXEC",
		Workdir: "/tmp",
		Env: map[string]string{
			"REXEC1": "VALUE1",
			"REXEC2": "VALUE2",
		},
	}

	if !reflect.DeepEqual(cmd, expectedCmd) {
		t.Errorf("Unmarshaled command does not match expected.\nGot: %#v\nWant: %#v", cmd, expectedCmd)
	}
}
