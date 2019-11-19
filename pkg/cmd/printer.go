package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ResultPrinter is an interface for writing data returned by a command.
type ResultPrinter interface {
	Print(v interface{}) error
}

// NoOpResultPrinter writes nothing.
type NoOpResultPrinter struct{}

// NoOpResultPrinter returns a new ResultPrinter that doesn't write anything.
func NewNoOpResultPrinter() ResultPrinter {
	return &NoOpResultPrinter{}
}

// Print implements ResultPrinter
func (p *NoOpResultPrinter) Print(v interface{}) error {
	return nil
}

// JSONResultPrinter writes data in JSON format.
type JSONResultPrinter struct {
	writer io.Writer
}

// NewJSONResultPrinter returns a new ResultPrinter that outputs JSON.
func NewJSONResultPrinter(writer io.Writer) ResultPrinter {
	return &JSONResultPrinter{writer: writer}
}

// Print implements ResultPrinter
func (p *JSONResultPrinter) Print(v interface{}) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(p.writer, string(buf))
	return err
}

// YAMLResultPrinter writes data in YAML format.
type YAMLResultPrinter struct {
	writer io.Writer
}

// NewYAMLResultPrinter returns a new ResultPrinter that outputs JSON.
func NewYAMLResultPrinter(writer io.Writer) ResultPrinter {
	return &YAMLResultPrinter{writer: writer}
}

// Print implements ResultPrinter
func (p *YAMLResultPrinter) Print(v interface{}) error {
	buf, err := yaml.Marshal(v)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(p.writer, string(buf))
	return err
}
