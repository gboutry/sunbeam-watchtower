package excusescache

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestDecodeRaw(t *testing.T) {
	t.Run("plain text", func(t *testing.T) {
		data := []byte("sources:\n  []\n")
		got, err := decodeRaw(data)
		if err != nil {
			t.Fatalf("decodeRaw() error = %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("decodeRaw() = %q, want %q", got, data)
		}
	})

	t.Run("xz", func(t *testing.T) {
		want := []byte("hello from xz")
		var buf bytes.Buffer
		writer, err := xz.NewWriter(&buf)
		if err != nil {
			t.Fatalf("xz.NewWriter() error = %v", err)
		}
		if _, err := writer.Write(want); err != nil {
			t.Fatalf("writer.Write() error = %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close() error = %v", err)
		}

		got, err := decodeRaw(buf.Bytes())
		if err != nil {
			t.Fatalf("decodeRaw() error = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("decodeRaw() = %q, want %q", got, want)
		}
	})

	t.Run("gzip", func(t *testing.T) {
		want := []byte("hello from gzip")
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		if _, err := writer.Write(want); err != nil {
			t.Fatalf("writer.Write() error = %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("writer.Close() error = %v", err)
		}

		got, err := decodeRaw(buf.Bytes())
		if err != nil {
			t.Fatalf("decodeRaw() error = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("decodeRaw() = %q, want %q", got, want)
		}
	})
}

func TestDetectCompression(t *testing.T) {
	if got := detectCompression([]byte{0xfd, '7', 'z', 'X', 'Z', 0x00, 0x01}); got != "xz" {
		t.Fatalf("detectCompression(xz) = %q, want xz", got)
	}
	if got := detectCompression([]byte{0x1f, 0x8b, 0x08, 0x00}); got != "gzip" {
		t.Fatalf("detectCompression(gzip) = %q, want gzip", got)
	}
	if got := detectCompression([]byte("plain")); got != "" {
		t.Fatalf("detectCompression(plain) = %q, want empty", got)
	}
}

func TestDecodeRawInvalidGzip(t *testing.T) {
	_, err := decodeRaw([]byte{0x1f, 0x8b, 0x00})
	if err == nil {
		t.Fatal("decodeRaw() expected error for invalid gzip")
	}
}

func TestDecodeRawInvalidXZ(t *testing.T) {
	_, err := decodeRaw([]byte{0xfd, '7', 'z', 'X', 'Z', 0x00, 0x00})
	if err == nil {
		t.Fatal("decodeRaw() expected error for invalid xz")
	}
}

func BenchmarkDecodeRawPlain(b *testing.B) {
	data := bytes.Repeat([]byte("plain text\n"), 128)
	for i := 0; i < b.N; i++ {
		got, err := decodeRaw(data)
		if err != nil {
			b.Fatalf("decodeRaw() error = %v", err)
		}
		if len(got) == 0 {
			b.Fatal("decodeRaw() returned empty result")
		}
	}
}

func BenchmarkDecodeRawGzip(b *testing.B) {
	want := bytes.Repeat([]byte("gzip text\n"), 128)
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(want); err != nil {
		b.Fatalf("writer.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		b.Fatalf("writer.Close() error = %v", err)
	}
	data, err := io.ReadAll(&buf)
	if err != nil {
		b.Fatalf("io.ReadAll() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := decodeRaw(data)
		if err != nil {
			b.Fatalf("decodeRaw() error = %v", err)
		}
		if len(got) == 0 {
			b.Fatal("decodeRaw() returned empty result")
		}
	}
}
