package parser

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var temporaryParagraphKey = NewContextKey()

type setextHeadingParser struct {
	HeadingConfig
}

func matchesSetextHeadingBar(line []byte) (byte, bool) {
	start := 0
	end := len(line)
	space := util.TrimLeftLength(line, []byte{' '})
	if space > 3 {
		return 0, false
	}
	start += space
	level1 := util.TrimLeftLength(line[start:end], []byte{'='})
	c := byte('=')
	var level2 int
	if level1 == 0 {
		level2 = util.TrimLeftLength(line[start:end], []byte{'-'})
		c = '-'
	}
	if util.IsSpace(line[end-1]) {
		end -= util.TrimRightSpaceLength(line[start:end])
	}
	if !((level1 > 0 && start+level1 == end) || (level2 > 0 && start+level2 == end)) {
		return 0, false
	}
	return c, true
}

// NewSetextHeadingParser return a new BlockParser that can parse Setext headings.
func NewSetextHeadingParser(opts ...HeadingOption) BlockParser {
	p := &setextHeadingParser{}
	for _, o := range opts {
		o.SetHeadingOption(&p.HeadingConfig)
	}
	return p
}

func (b *setextHeadingParser) Open(parent ast.Node, reader text.Reader, pc Context) (ast.Node, State) {
	last := pc.LastOpenedBlock().Node
	if last == nil {
		return nil, NoChildren
	}
	paragraph, ok := last.(*ast.Paragraph)
	if !ok || paragraph.Parent() != parent {
		return nil, NoChildren
	}
	line, segment := reader.PeekLine()
	c, ok := matchesSetextHeadingBar(line)
	if !ok {
		return nil, NoChildren
	}
	level := 1
	if c == '-' {
		level = 2
	}
	node := ast.NewHeading(level)
	node.Lines().Append(segment)
	pc.Set(temporaryParagraphKey, paragraph)
	return node, NoChildren
}

func (b *setextHeadingParser) Continue(node ast.Node, reader text.Reader, pc Context) State {
	return Close
}

func (b *setextHeadingParser) Close(node ast.Node, reader text.Reader, pc Context) {
	heading := node.(*ast.Heading)
	segment := node.Lines().At(0)
	heading.Lines().Clear()
	tmp := pc.Get(temporaryParagraphKey).(*ast.Paragraph)
	pc.Set(temporaryParagraphKey, nil)
	if tmp.Lines().Len() == 0 {
		next := heading.NextSibling()
		segment = segment.TrimLeftSpace(reader.Source())
		if next == nil || !ast.IsParagraph(next) {
			para := ast.NewParagraph()
			para.Lines().Append(segment)
			heading.Parent().InsertAfter(heading.Parent(), heading, para)
		} else {
			next.(ast.Node).Lines().Unshift(segment)
		}
		heading.Parent().RemoveChild(heading.Parent(), heading)
	} else {
		heading.SetLines(tmp.Lines())
		heading.SetBlankPreviousLines(tmp.HasBlankPreviousLines())
		tmp.Parent().RemoveChild(tmp.Parent(), tmp)
	}

	if b.Attribute {
		lastIndex := node.Lines().Len() - 1
		lastLine := node.Lines().At(lastIndex)
		line := lastLine.Value(reader.Source())
		indicies := util.FindAttributeIndiciesReverse(line, true)
		if indicies != nil {
			for _, index := range indicies {
				node.SetAttribute(line[index[0]:index[1]], line[index[2]:index[3]])
			}
			lastLine.Stop = lastLine.Start + indicies[0][0] - 1
			lastLine.TrimRightSpace(reader.Source())
			node.Lines().Set(lastIndex, lastLine)
		}
	}

	if b.AutoHeadingID {
		_, ok := node.AttributeString("id")
		if !ok {
			generateAutoHeadingID(heading, reader, pc)
		}
	}
}

func (b *setextHeadingParser) CanInterruptParagraph() bool {
	return true
}

func (b *setextHeadingParser) CanAcceptIndentedLine() bool {
	return false
}
