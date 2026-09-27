package signaltest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"
)

type Golden struct {
	Seeds   int      `json:"seeds"`
	Rounds  int      `json:"rounds"`
	Digests []string `json:"digests"`
}

func LoadGolden(path string, rounds int) (Golden, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Golden{}, fmt.Errorf("signaltest: %w", err)
	}
	var g Golden
	if err := json.Unmarshal(raw, &g); err != nil {
		return Golden{}, fmt.Errorf("signaltest: %s: %w", path, err)
	}
	if g.Rounds != rounds || g.Seeds != len(g.Digests) || g.Seeds == 0 {
		return Golden{}, fmt.Errorf("signaltest: %s holds %d rounds and %d of %d seeds, want %d rounds", path, g.Rounds, len(g.Digests), g.Seeds, rounds)
	}
	return g, nil
}

func (g Golden) Write(path string) error {
	encoded, err := json.Marshal(g, jsontext.WithIndent("  "))
	if err != nil {
		return fmt.Errorf("signaltest: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("signaltest: %w", err)
	}
	return nil
}
