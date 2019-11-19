package yaml

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

const (
	strictMode  = true
	indentWidth = 2
)

// yamlCodec provides the YAML implementation of orgbot.Codec.
type yamlCodec struct{}

// NewCodec returns a new YAML Codec implementation.
func NewCodec() *yamlCodec {
	return &yamlCodec{}
}

// Decode implements orgbot.Codec.
func (c *yamlCodec) Decode(in []byte, v interface{}) error {
	r := bytes.NewReader(in)
	d := yaml.NewDecoder(r)
	d.KnownFields(strictMode)
	return d.Decode(v)
}

// Encode implements orgbot.Codec.
func (c *yamlCodec) Encode(in interface{}) ([]byte, error) {
	var buf bytes.Buffer
	e := yaml.NewEncoder(&buf)
	e.SetIndent(indentWidth)

	if err := e.Encode(in); err != nil {
		return nil, err
	}

	if err := e.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
