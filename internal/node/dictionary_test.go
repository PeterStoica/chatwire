package node_test

import (
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
)

func TestNewDictionaryLimits(t *testing.T) {
	t.Parallel()
	tables := func(version byte, single, doubleTables, doubleTokens int) node.Tables {
		out := node.Tables{Version: version, Single: make([]string, single)}
		for range doubleTables {
			out.Double = append(out.Double, make([]string, doubleTokens))
		}
		return out
	}
	tests := []struct {
		name   string
		tables node.Tables
		valid  bool
	}{
		{name: "at every limit", tables: tables(3, 236, 4, 256), valid: true},
		{name: "version zero", tables: tables(0, 1, 1, 1)},
		{name: "237 single tokens would reach the dictionary tags", tables: tables(3, 237, 1, 1)},
		{name: "5 double tables", tables: tables(3, 1, 5, 1)},
		{name: "257 tokens in a double table", tables: tables(3, 1, 1, 257)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dict, err := node.NewDictionary(tt.tables)
			if tt.valid {
				if err != nil || dict.Version() != tt.tables.Version {
					t.Fatalf("NewDictionary() = v%d, %v", dict.Version(), err)
				}
				return
			}
			if !errors.Is(err, node.ErrDictionary) {
				t.Fatalf("NewDictionary() error = %v, want %v", err, node.ErrDictionary)
			}
		})
	}
}

func TestEmbeddedDictionary(t *testing.T) {
	dict := dictionary(t)
	if dict.Version() != 3 {
		t.Fatalf("embedded dictionary version = %d, want 3", dict.Version())
	}
}
