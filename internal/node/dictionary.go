package node

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	maxSingleTokens = tagDictionary0
	maxDoubleTables = 4
	maxDoubleTokens = 256
)

var ErrDictionary = errors.New("node: invalid token dictionary")

//go:embed tokens.json
var tokensJSON []byte

type Tables struct {
	Version byte       `json:"version"`
	Single  []string   `json:"single"`
	Double  [][]string `json:"double"`
}

type doubleIndex struct {
	table byte
	index byte
}

type Dictionary struct {
	version     byte
	single      []string
	double      [][]string
	singleIndex map[string]byte
	doubleIndex map[string]doubleIndex
}

func LoadDictionary() (Dictionary, error) {
	var tables Tables
	if err := json.Unmarshal(tokensJSON, &tables); err != nil {
		return Dictionary{}, fmt.Errorf("%w: %w", ErrDictionary, err)
	}
	return NewDictionary(tables)
}

func NewDictionary(tables Tables) (Dictionary, error) {
	if tables.Version == 0 || len(tables.Single) > maxSingleTokens || len(tables.Double) > maxDoubleTables {
		return Dictionary{}, fmt.Errorf("%w: version %d, %d single tokens, %d double tables",
			ErrDictionary, tables.Version, len(tables.Single), len(tables.Double))
	}
	d := Dictionary{
		version:     tables.Version,
		single:      tables.Single,
		double:      tables.Double,
		singleIndex: map[string]byte{},
		doubleIndex: map[string]doubleIndex{},
	}
	for i, token := range tables.Single {
		if token != "" {
			d.singleIndex[token] = byte(i)
		}
	}
	for table, tokens := range tables.Double {
		if len(tokens) > maxDoubleTokens {
			return Dictionary{}, fmt.Errorf("%w: double table %d has %d tokens", ErrDictionary, table, len(tokens))
		}
		for i, token := range tokens {
			d.doubleIndex[token] = doubleIndex{table: byte(table), index: byte(i)}
		}
	}
	return d, nil
}

func (d Dictionary) Version() byte {
	return d.version
}
