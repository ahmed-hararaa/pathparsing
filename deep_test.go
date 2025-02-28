package pathparsing

import (
	"fmt"
	"testing"
)

type DeepTestPathProxy struct {
	expectedCommands []string
	actualCommands   []string
}

func NewDeepTestPathProxy(expectedCommands []string) *DeepTestPathProxy {
	return &DeepTestPathProxy{
		expectedCommands: expectedCommands,
		actualCommands:   []string{},
	}
}

func (p *DeepTestPathProxy) MoveTo(x, y float64) {
	p.actualCommands =
		append(p.actualCommands, fmt.Sprintf("moveTo(%.4f, %.4f)", x, y))
}

func (p *DeepTestPathProxy) LineTo(x, y float64) {
	p.actualCommands =
		append(p.actualCommands, fmt.Sprintf("lineTo(%.4f, %.4f)", x, y))
}

func (p *DeepTestPathProxy) CubicTo(x1, y1, x2, y2, x3, y3 float64) {
	p.actualCommands =
		append(p.actualCommands, fmt.Sprintf("cubicTo(%.4f, %.4f, %.4f, %.4f, %.4f, %.4f)", x1, y1, x2, y2, x3, y3))
}

func (p *DeepTestPathProxy) Close() {
	p.actualCommands =
		append(p.actualCommands, "close()")
}

func (p *DeepTestPathProxy) Validate() {
	if len(p.expectedCommands) != len(p.actualCommands) {
		panic("Not equal")
	}

	for i := 0; i < len(p.expectedCommands); i++ {
		if p.expectedCommands[i] != p.actualCommands[i] {
			panic("Not equal")
		}
	}
}

func assertValidPathDeep(input string, commands []string) {
	proxy := NewDeepTestPathProxy(commands)
	WriteSvgPathDataToPath(input, proxy)
	proxy.Validate()
}

func TestParsePathDeepTest(t *testing.T) {

	assertValidPathDeep("M20,30 Q40,5 60,30 T100,30", []string{
		"moveTo(20.0000, 30.0000)",
		"cubicTo(33.3333, 13.3333, 46.6667, 13.3333, 60.0000, 30.0000)",
		"cubicTo(73.3333, 46.6667, 86.6667, 46.6667, 100.0000, 30.0000)",
	})

	assertValidPathDeep("M5.5 5.5a.5 1.5 30 1 1-.866-.5.5 1.5 30 1 1 .866.5z", []string{
		"moveTo(5.5000, 5.5000)",
		"cubicTo(5.2319, 5.9667, 4.9001, 6.3513, 4.6307, 6.5077)",
		"cubicTo(4.3612, 6.6640, 4.1953, 6.5683, 4.1960, 6.2567)",
		"cubicTo(4.1967, 5.9451, 4.3638, 5.4655, 4.6340, 5.0000)",
		"cubicTo(4.9021, 4.5333, 5.2339, 4.1487, 5.5033, 3.9923)",
		"cubicTo(5.7728, 3.8360, 5.9387, 3.9317, 5.9380, 4.2433)",
		"cubicTo(5.9373, 4.5549, 5.7702, 5.0345, 5.5000, 5.5000)",
		"close()",
	})
}
