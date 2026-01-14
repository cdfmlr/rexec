package rexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
)

func TestManagedIO_Hijack(t *testing.T) {
	oldStdin := bytes.NewBufferString("oldStdin:")
	oldStdout := bytes.NewBufferString("oldStdout:")
	oldStderr := bytes.NewBufferString("oldStderr:")

	newStdin := bytes.NewBufferString("newStdin:")
	newStdout := bytes.NewBufferString("newStdout:")
	newStderr := bytes.NewBufferString("newStderr:")

	// reset buffers to initial state for each test run
	resetBuffers := func() {
		oldStdin.Reset()
		oldStdin.WriteString("oldStdin:")

		oldStdout.Reset()
		oldStdout.WriteString("oldStdout:")

		oldStderr.Reset()
		oldStderr.WriteString("oldStderr:")

		newStdin.Reset()
		newStdin.WriteString("newStdin:")

		newStdout.Reset()
		newStdout.WriteString("newStdout:")

		newStderr.Reset()
		newStderr.WriteString("newStderr:")
	}

	var mu sync.Mutex // no concurrent tests

	t.Run("nilCmd", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		resetBuffers()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("❌ Hijack() panic: %v.", r)
			}
		}()

		m := &ManagedIO{
			Stdin:  newStdin,
			Stdout: newStdout,
			Stderr: newStderr,
		}
		var cmd *Command = nil

		m.Hijack(cmd)

		if cmd != nil {
			t.Errorf("Hijack() cmd = %#v, want nil", cmd)
		}
	})

	type fields struct {
		Stdin  *bytes.Buffer
		Stdout *bytes.Buffer
		Stderr *bytes.Buffer
	}
	type args struct {
		cmd *Command
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"nilAllIO",
			fields{
				Stdin:  newStdin,
				Stdout: newStdout,
				Stderr: newStderr,
			},
			args{
				&Command{
					Stdin:  nil,
					Stdout: nil,
					Stderr: nil,
				},
			},
		},
		{
			"nonNilAllIO",
			fields{
				Stdin:  newStdin,
				Stdout: newStdout,
				Stderr: newStderr,
			},
			args{
				&Command{
					Stdin:  oldStdin,
					Stdout: oldStdout,
					Stderr: oldStderr,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mu.Lock()
			defer mu.Unlock()

			resetBuffers()

			var hijackedStdin io.Reader = nil
			var hijackedStdout io.Writer = nil
			var hijackedStderr io.Writer = nil

			if tt.args.cmd.Stdin != nil {
				hijackedStdin = tt.args.cmd.Stdin
			}
			if tt.args.cmd.Stdout != nil {
				hijackedStdout = tt.args.cmd.Stdout
			}
			if tt.args.cmd.Stderr != nil {
				hijackedStderr = tt.args.cmd.Stderr
			}

			m := &ManagedIO{
				Stdin:  tt.fields.Stdin,
				Stdout: tt.fields.Stdout,
				Stderr: tt.fields.Stderr,
			}

			m.Hijack(tt.args.cmd)

			// values check

			if tt.args.cmd.Stdin != newStdin {
				t.Errorf("❌ Hijack() Stdin = %T(%q), want %T(%q)", tt.args.cmd.Stdin, tt.args.cmd.Stdin, newStdin, newStdin)
			} else {
				t.Logf("✅ Hijack() Stdin = %T(%q)", tt.args.cmd.Stderr, tt.args.cmd.Stderr)
			}
			if tt.args.cmd.Stdout != newStdout {
				t.Errorf("❌ Hijack() Stdout = %T(%q), want %T(%q)", tt.args.cmd.Stdout, tt.args.cmd.Stdout, newStdout, newStdout)
			} else {
				t.Logf("✅ Hijack() Stdout = %T(%q)", tt.args.cmd.Stdout, tt.args.cmd.Stdout)
			}
			if tt.args.cmd.Stderr != newStderr {
				t.Errorf("❌ Hijack() Stderr = %T(%q), want %T(%q)", tt.args.cmd.Stderr, tt.args.cmd.Stderr, newStderr, newStderr)
			} else {
				t.Logf("✅ Hijack() Stderr = %T(%q)", tt.args.cmd.Stderr, tt.args.cmd.Stderr)
			}

			// r/w pass-through check

			if hijackedStdin != nil {
				// phase 1: write to old Stdin

				if oldStdin.String() != "oldStdin:" {
					t.Fatalf("❌ Hijack() old Stdin r/w(old) old = %q, want %q", oldStdin.String(), "oldStdin:")
				}

				_, err := oldStdin.WriteString("hello")
				if err != nil {
					t.Errorf("❌ Write old Stdin got error: %v", err)
				}

				gotOldBuffer := oldStdin.String()
				expected := "oldStdin:hello"

				if gotOldBuffer != expected {
					t.Errorf("❌ Hijack() old Stdin r/w(old) gotOldBuffer = %q, want %q", gotOldBuffer, expected)
				} else {
					t.Logf("✅ Hijack() Stdin r/w(old) gotOldBuffer == expected (%q)", expected)
				}

				expected = "newStdin:"

				gotRawBuffer := tt.fields.Stdin.String()
				gotManagedIOField := m.Stdin.String()
				gotCmdField, err := io.ReadAll(tt.args.cmd.Stdin)

				if err != nil {
					t.Errorf("❌ Read hijacked Cmd Stdin got error: %v", err)
				}

				for name, got := range map[string]string{
					"gotRawBuffer":      gotRawBuffer,
					"gotManagedIOField": gotManagedIOField,
					"gotCmdField":       string(gotCmdField),
				} {
					if got != expected {
						t.Errorf("❌ Hijack() Stdin r/w(old) %v = %q, want %q", name, got, expected)
					} else {
						t.Logf("✅ Hijack() Stdin r/w(old) %v == expected (%q)", name, expected)
					}
				}

				// phase 2: write to new Stdin

				_, err = m.Stdin.Write([]byte("world"))
				if err != nil {
					t.Errorf("❌ Write hijacked ManagedIO Stdin got error: %v", err)
				}

				gotOldBuffer = oldStdin.String()
				expected = "oldStdin:hello"
				if gotOldBuffer != expected {
					t.Errorf("❌ Hijack() Stdin r/w(new) gotOldBuffer = %q, want %q", gotOldBuffer, expected)
				} else {
					t.Logf("✅ Hijack() Stdin r/w(new) gotOldBuffer == expected (%q)", expected)
				}

				gotManagedIOField = m.Stdin.String()
				gotCmdField, err = io.ReadAll(tt.args.cmd.Stdin)
				if err != nil {
					t.Errorf("❌ Read hijacked Cmd Stdin got error: %v", err)
				}

				// the "newStdin:" is consumed by the io.ReadAll(tt.args.cmd.Stdin) call at phase 1 above
				expected = "world"

				for name, got := range map[string]string{
					"gotManagedIOField": gotManagedIOField,
					"gotCmdField":       string(gotCmdField),
				} {
					if got != expected {
						t.Errorf("❌ Hijack() Stdin r/w(new) %v = %q, want %q", name, got, expected)
					} else {
						t.Logf("✅ Hijack() Stdin r/w(new) %v == expected (%q)", name, expected)
					}
				}
			}

			if hijackedStdout != nil {
				_, err := tt.args.cmd.Stdout.Write([]byte("hello"))
				if err != nil {
					t.Errorf("❌ Write hijacked Cmd Stdout got error: %v", err)
				}

				gotOldBuffer := oldStdout.String()
				gotHijackedBuffer := hijackedStdout.(*bytes.Buffer).String()
				expected := "oldStdout:"

				for name, got := range map[string]string{
					"gotOldBuffer":      gotOldBuffer,
					"gotHijackedBuffer": gotHijackedBuffer,
				} {
					if got != expected {
						t.Errorf("❌ Hijack() Stdout r/w %v = %q, want %q", name, got, expected)
					} else {
						t.Logf("✅ Hijack() Stdout r/w %v == expected (%q)", name, expected)
					}
				}

				gotRawBuffer := tt.fields.Stdout.String()
				gotManagedIOField := m.Stdout.String()
				expected = "newStdout:hello"

				for name, got := range map[string]string{
					"gotRawBuffer":      gotRawBuffer,
					"gotManagedIOField": gotManagedIOField,
				} {
					if got != expected {
						t.Errorf("❌ Hijack() Stdout r/w %v = %q, want %q", name, got, expected)
					} else {
						t.Logf("✅ Hijack() Stdout r/w %v == expected (%q)", name, expected)
					}
				}
			}

			if hijackedStderr != nil {
				_, err := tt.args.cmd.Stderr.Write([]byte("hello"))
				if err != nil {
					t.Errorf("❌ Write hijacked Cmd Stderr got error: %v", err)
				}

				gotOldBuffer := oldStderr.String()
				gotHijackedBuffer := hijackedStderr.(*bytes.Buffer).String()
				expected := "oldStderr:"

				for name, got := range map[string]string{
					"gotOldBuffer":      gotOldBuffer,
					"gotHijackedBuffer": gotHijackedBuffer,
				} {
					if got != expected {
						t.Errorf("❌ Hijack() Stderr r/w %v = %q, want %q", name, got, expected)
					} else {
						t.Logf("✅ Hijack() Stderr r/w %v == expected (%q)", name, expected)
					}
				}

				gotRawBuffer := tt.fields.Stderr.String()
				gotManagedIOField := m.Stderr.String()
				expected = "newStderr:hello"

				for name, got := range map[string]string{
					"gotRawBuffer":      gotRawBuffer,
					"gotManagedIOField": gotManagedIOField,
				} {
					if got != expected {
						t.Errorf("❌ Hijack() Stderr r/w %v = %q, want %q", name, got, expected)
					} else {
						t.Logf("✅ Hijack() Stderr r/w %v == expected (%q)", name, expected)
					}
				}
			}
		})
	}
}

func TestManagedIO_makeNonNil(t *testing.T) {
	notNilBuffers := map[string]*bytes.Buffer{
		"Stdin":  bytes.NewBufferString("stdin"),
		"Stdout": bytes.NewBufferString("stdout"),
		"Stderr": bytes.NewBufferString("stderr"),
	}

	type fields struct {
		Stdin  *bytes.Buffer
		Stdout *bytes.Buffer
		Stderr *bytes.Buffer
	}
	tests := []struct {
		name      string
		fields    fields
		nilFields []string
	}{
		{
			"allNil",
			fields{
				Stdin:  nil,
				Stdout: nil,
				Stderr: nil,
			},
			[]string{"Stdin", "Stdout", "Stderr"},
		},
		{
			"StdinNil",
			fields{
				Stdin:  nil,
				Stdout: notNilBuffers["Stdout"],
				Stderr: notNilBuffers["Stderr"],
			},
			[]string{"Stdin"},
		},
		{
			"StdoutNil",
			fields{
				Stdin:  notNilBuffers["Stdin"],
				Stdout: nil,
				Stderr: notNilBuffers["Stderr"],
			},
			[]string{"Stdout"},
		},
		{
			"StderrNil",
			fields{
				Stdin:  notNilBuffers["Stdin"],
				Stdout: notNilBuffers["Stdout"],
				Stderr: nil,
			},
			[]string{"Stderr"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ManagedIO{
				Stdin:  tt.fields.Stdin,
				Stdout: tt.fields.Stdout,
				Stderr: tt.fields.Stderr,
			}
			m.makeNonNil()

			if m.Stdin == nil {
				t.Errorf("makeNonNil() Stdin = nil")
			}
			if m.Stdout == nil {
				t.Errorf("makeNonNil() Stdout = nil")
			}
			if m.Stderr == nil {
				t.Errorf("makeNonNil() Stderr = nil")
			}

			for _, nilField := range tt.nilFields {
				if m.Stdin == notNilBuffers[nilField] {
					t.Errorf("makeNonNil() Stdin = %#v, want %#v", m.Stdin, &bytes.Buffer{})
				}
				if m.Stdout == notNilBuffers[nilField] {
					t.Errorf("makeNonNil() Stdout = %#v, want %#v", m.Stdout, &bytes.Buffer{})
				}
				if m.Stderr == notNilBuffers[nilField] {
					t.Errorf("makeNonNil() Stderr = %#v, want %#v", m.Stderr, &bytes.Buffer{})
				}
			}
		})
	}
}

func TestNewCombinedOutputManagedIO(t *testing.T) {
	got := NewCombinedOutputManagedIO()

	if got.Stdin == nil {
		t.Errorf("NewCombinedOutputManagedIO() Stdin = nil")
	}
	if got.Stdout == nil {
		t.Errorf("NewCombinedOutputManagedIO() Stdout = nil")
	}
	if got.Stderr == nil {
		t.Errorf("NewCombinedOutputManagedIO() Stderr = nil")
	}
	if got.Stdout != got.Stderr {
		t.Errorf("NewCombinedOutputManagedIO() Stdout != Stderr")
	}
}

func TestNewManagedIO(t *testing.T) {
	tests := []struct {
		name string
		want *ManagedIO
	}{
		{
			"NewManagedIO",
			&ManagedIO{
				Stdin:  &bytes.Buffer{},
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewManagedIO(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewManagedIO() = %v, want %v", got, tt.want)
			}
		})
	}
}

// This test is a playground for understanding how io.Copy works.
//
// It indicates that Copy consumes the src buffer and writes it to the dst buffer.
// So we can't depend on it to pass through the data from ManagedIO to the
// hijacked IO.
func Test_ioCopy(t *testing.T) {
	t.Skip("🚗 Skip.")

	var src, dst *bytes.Buffer

	src = bytes.NewBufferString("src0:")
	dst = bytes.NewBufferString("dst0:")

	assertEqual := func(got, want string) {
		if got != want {
			t.Errorf("❌ got = %q, want %q", got, want)
		}
	}

	t.Logf("main Write 0: src=%q, dst=%q", src.String(), dst.String())
	assertEqual(src.String(), "src0:")
	assertEqual(dst.String(), "dst0:")

	src.WriteString("SRC1:")
	dst.WriteString("DST1:")

	t.Logf("main Write 1: src=%q, dst=%q", src.String(), dst.String())
	assertEqual(src.String(), "src0:SRC1:")
	assertEqual(dst.String(), "dst0:DST1:")

	done := make(chan struct{})
	go func() {
		defer func() {
			t.Logf("Copy return: src=%q, dst=%q", src.String(), dst.String())
			assertEqual(src.String(), "src0:SRC1:src2:")
			assertEqual(dst.String(), "dst0:DST1:src0:SRC1:src2:dst2:")

			close(done)
		}()

		_, _ = io.Copy(dst, src)
	}()

	src.WriteString("src2:")
	dst.WriteString("dst2:")

	t.Logf("main Write 2: src=%q, dst=%q", src.String(), dst.String())
	assertEqual(src.String(), "src0:SRC1:src2:")
	assertEqual(dst.String(), "dst0:DST1:src0:SRC1:src2:dst2:")

	<-done

	t.Logf("main Join copy goruntine: src=%q, dst=%q", src.String(), dst.String())
	assertEqual(src.String(), "src0:SRC1:src2:")
	assertEqual(dst.String(), "dst0:DST1:src0:SRC1:src2:dst2:")
}

func ExampleNewManagedIO() {
	// create a new Command
	cmd := &Command{
		Command: "cat -", // this command reads from stdin and writes to stdout
	}

	// hijack the command's IO
	m := NewManagedIO()
	m.Hijack(cmd)

	// write to the hijacked stdin
	m.Stdin.Write([]byte("hello"))

	// execute the command
	executor := &LocalExecutor{}
	err := executor.Execute(context.Background(), cmd)
	if err != nil {
		panic(err)
	}

	// read from the hijacked stdout
	out, err := io.ReadAll(m.Stdout)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
	// Output: hello
}
