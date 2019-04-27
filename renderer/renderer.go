// Package renderer	renders given AST to certain formats.
package renderer

import (
	"bufio"
	"io"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"

	"sync"
)

// A Config struct is a data structure that holds configuration of the Renderer.
type Config struct {
	Options       map[OptionName]interface{}
	NodeRenderers util.PrioritizedSlice
}

// NewConfig returns a new Config
func NewConfig() *Config {
	return &Config{
		Options:       map[OptionName]interface{}{},
		NodeRenderers: util.PrioritizedSlice{},
	}
}

type notSupported struct {
}

func (e *notSupported) Error() string {
	return "not supported by this parser"
}

// NotSupported indicates given node can not be rendered by this NodeRenderer.
var NotSupported = &notSupported{}

// An OptionName is a name of the option.
type OptionName string

// An Option interface is a functional option type for the Renderer.
type Option interface {
	SetConfig(*Config)
}

type withNodeRenderers struct {
	value []util.PrioritizedValue
}

func (o *withNodeRenderers) SetConfig(c *Config) {
	c.NodeRenderers = append(c.NodeRenderers, o.value...)
}

// WithNodeRenderers is a functional option that allow you to add
// NodeRenderers to the renderer.
func WithNodeRenderers(ps ...util.PrioritizedValue) Option {
	return &withNodeRenderers{ps}
}

type withOption struct {
	name  OptionName
	value interface{}
}

func (o *withOption) SetConfig(c *Config) {
	c.Options[o.name] = o.value
}

// WithOption is a functional option that allow you to set
// an arbitary option to the parser.
func WithOption(name OptionName, value interface{}) Option {
	return &withOption{name, value}
}

// A SetOptioner interface sets given option to the object.
type SetOptioner interface {
	// SetOption sets given option to the object.
	// Unacceptable options may be passed.
	// Thus implementations must ignore unacceptable options.
	SetOption(name OptionName, value interface{})
}

// A NodeRenderer interface renders given AST node to given writer.
type NodeRenderer interface {
	// Render renders given AST node to given writer.
	Render(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error)
}

// A Renderer interface renders given AST node to given
// writer with given Renderer.
type Renderer interface {
	Render(w io.Writer, source []byte, n ast.Node) error

	// AddOption adds given option to thie parser.
	AddOption(Option)
}

type renderer struct {
	config        *Config
	options       map[OptionName]interface{}
	nodeRenderers []NodeRenderer
	initSync      sync.Once
}

// NewRenderer returns a new Renderer with given options.
func NewRenderer(options ...Option) Renderer {
	config := NewConfig()
	for _, opt := range options {
		opt.SetConfig(config)
	}

	r := &renderer{
		options: map[OptionName]interface{}{},
		config:  config,
	}

	return r
}

func (r *renderer) AddOption(o Option) {
	o.SetConfig(r.config)
}

// Render renders given AST node to given writer with given Renderer.
func (r *renderer) Render(w io.Writer, source []byte, n ast.Node) error {
	r.initSync.Do(func() {
		r.options = r.config.Options
		r.config.NodeRenderers.Sort()
		r.nodeRenderers = make([]NodeRenderer, 0, len(r.config.NodeRenderers))
		for _, v := range r.config.NodeRenderers {
			nr, _ := v.Value.(NodeRenderer)
			if se, ok := v.Value.(SetOptioner); ok {
				for oname, ovalue := range r.options {
					se.SetOption(oname, ovalue)
				}
			}
			r.nodeRenderers = append(r.nodeRenderers, nr)
		}
		r.config = nil
	})
	writer, ok := w.(util.BufWriter)
	if !ok {
		writer = bufio.NewWriter(w)
	}
	err := ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		var s ast.WalkStatus
		var err error
		for _, nr := range r.nodeRenderers {
			s, err = nr.Render(writer, source, n, entering)
			if err == NotSupported {
				continue
			}
			break
		}
		return s, err
	})
	if err != nil {
		return err
	}
	return writer.Flush()
}