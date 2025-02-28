package pathparsing

import (
	"errors"
	"fmt"
	"github.com/go-gl/mathgl/mgl32"
	"math"
	"unicode"
	//"unicode"
)

// PathProxy defines the interface for path operations.
type PathProxy interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
	CubicTo(x1, y1, x2, y2, x3, y3 float64)
	Close()
}

// PathOffset represents a 2D point with X and Y coordinates.
type PathOffset struct {
	Dx, Dy float64
}

// ZeroPathOffset returns a PathOffset with zero values.
func ZeroPathOffset() PathOffset {
	return PathOffset{0.0, 0.0}
}

// Direction returns the angle in radians of the vector.
func (p PathOffset) Direction() float64 {
	return math.Atan2(p.Dy, p.Dx)
}

// Translate returns a new PathOffset translated by the given amounts.
func (p PathOffset) Translate(translateX, translateY float64) PathOffset {
	return PathOffset{p.Dx + translateX, p.Dy + translateY}
}

// Add returns the sum of two PathOffsets.
func (p PathOffset) Add(other PathOffset) PathOffset {
	return PathOffset{p.Dx + other.Dx, p.Dy + other.Dy}
}

// Subtract returns the difference between two PathOffsets.
func (p PathOffset) Subtract(other PathOffset) PathOffset {
	return PathOffset{p.Dx - other.Dx, p.Dy - other.Dy}
}

// Multiply returns the PathOffset scaled by the given factor.
func (p PathOffset) Multiply(operand float64) PathOffset {
	return PathOffset{p.Dx * operand, p.Dy * operand}
}

// String returns a string representation of the PathOffset.
func (p PathOffset) String() string {
	return fmt.Sprintf("PathOffset{%f,%f}", p.Dx, p.Dy)
}

func mapPoint(transform mgl32.Mat4, point PathOffset) PathOffset {
	return PathOffset{
		float64(transform[0])*point.Dx + float64(transform[4])*point.Dy + float64(transform[12]),
		float64(transform[1])*point.Dx + float64(transform[5])*point.Dy + float64(transform[13]),
	}
}

// SvgPathParser parses SVG path data and writes it to a path.

// WriteSvgPathDataToPath writes SVG path data to the given path.
func WriteSvgPathDataToPath(svg string, path PathProxy) error {
	if svg == "" {
		return nil
	}

	parser := newSvgPathStringSource(svg)
	normalizer := NewSvgPathNormalizer()
	for parser.hasMoreData() {
		seg, err := parser.parseSegment()
		if err != nil {
			return err
		}
		normalizer.emitSegment(seg, path)
	}
	return nil
}

// SvgPathStringSource is a source of SVG path data.
type SvgPathStringSource struct {
	str             string
	previousCommand SvgPathSegType
	idx             int
	length          int
}

// newSvgPathStringSource creates a new SvgPathStringSource.
func newSvgPathStringSource(s string) *SvgPathStringSource {
	res := &SvgPathStringSource{
		str:    s,
		idx:    0,
		length: len(s),
	}
	res.skipOptionalSvgSpaces()
	return res
}

// isHtmlSpace checks if a character is an HTML space.
func (s *SvgPathStringSource) isHtmlSpace(c rune) bool {
	return c <= 32 && (c == 32 || c == 10 || c == 9 || c == 13 || c == 12)
}

// skipOptionalSvgSpaces skips optional spaces in the SVG string.
func (s *SvgPathStringSource) skipOptionalSvgSpaces() rune {
	for {
		if s.idx >= s.length {
			return -1
		}
		c := rune(s.str[s.idx])
		if !s.isHtmlSpace(c) {
			return c
		}
		s.idx++
	}
}

// skipOptionalSvgSpacesOrDelimiter skips optional spaces or a delimiter.
func (s *SvgPathStringSource) skipOptionalSvgSpacesOrDelimiter(delimiter rune) {
	c := s.skipOptionalSvgSpaces()
	if c == delimiter {
		s.idx++
		s.skipOptionalSvgSpaces()
	}
}

// isNumberStart checks if a character is the start of a number.
func (s *SvgPathStringSource) isNumberStart(c rune) bool {
	return unicode.IsDigit(c) || c == '+' || c == '-' || c == '.'
}

// maybeImplicitCommand determines the implicit command.
func (s *SvgPathStringSource) maybeImplicitCommand(lookahead rune, nextCommand SvgPathSegType) SvgPathSegType {
	if !s.isNumberStart(lookahead) || s.previousCommand == SvgPathSegTypeClose {
		return nextCommand
	}
	if s.previousCommand == SvgPathSegTypeMoveToAbs {
		return SvgPathSegTypeLineToAbs
	}
	if s.previousCommand == SvgPathSegTypeMoveToRel {
		return SvgPathSegTypeLineToRel
	}
	return s.previousCommand
}

// readCodeUnit reads the next character from the string.
func (s *SvgPathStringSource) readCodeUnit() rune {
	if s.idx >= s.length {
		return -1
	}
	c := rune(s.str[s.idx])
	s.idx++
	return c
}

// parseNumber parses a number from the string.
func (s *SvgPathStringSource) parseNumber() (float64, error) {
	s.skipOptionalSvgSpaces()

	sign := 1.0
	c := s.readCodeUnit()
	if c == '+' {
		c = s.readCodeUnit()
	} else if c == '-' {
		sign = -1.0
		c = s.readCodeUnit()
	}

	if (c < '0' || c > '9') && c != '.' {
		return 0, errors.New("first character of a number must be one of [0-9+-.]")
	}

	integer := 0.0
	for '0' <= c && c <= '9' {
		integer = integer*10 + float64(c-'0')
		c = s.readCodeUnit()
	}

	if !isValidRange(integer) {
		return 0, errors.New("numeric overflow")
	}

	decimalPart := 0.0
	if c == '.' {
		c = s.readCodeUnit()

		if c < '0' || c > '9' {
			return 0, errors.New("there must be at least one digit following the ")
		}

		frac := 1.0
		for '0' <= c && c <= '9' {
			frac *= 0.1
			decimalPart += float64(c-'0') * frac
			c = s.readCodeUnit()
		}
	}

	number := integer + decimalPart
	number *= sign

	if s.idx < s.length && (c == 'e' || c == 'E') && (s.str[s.idx] != 'x' && s.str[s.idx] != 'm') {
		c = s.readCodeUnit()

		exponentIsNegative := false
		if c == '+' {
			c = s.readCodeUnit()
		} else if c == '-' {
			c = s.readCodeUnit()
			exponentIsNegative = true
		}

		if c < '0' || c > '9' {
			return 0, errors.New("missing exponent")
		}

		exponent := 0.0
		for c >= '0' && c <= '9' {
			exponent *= 10.0
			exponent += float64(c - '0')
			c = s.readCodeUnit()
		}
		if exponentIsNegative {
			exponent = -exponent
		}
		if !isValidExponent(exponent) {
			return 0, fmt.Errorf("invalid exponent %f", exponent)
		}
		if exponent != 0 {
			number *= math.Pow(10.0, exponent)
		}
	}

	if !isValidRange(number) {
		return 0, errors.New("numeric overflow")
	}

	if c != -1 {
		s.idx--
		s.skipOptionalSvgSpacesOrDelimiter(',')
	}
	return number, nil
}

// parseArcFlag parses an arc flag from the string.
func (s *SvgPathStringSource) parseArcFlag() (bool, error) {
	if !s.hasMoreData() {
		return false, errors.New("expected more data")
	}
	flagChar := s.str[s.idx]
	s.idx++
	s.skipOptionalSvgSpacesOrDelimiter(',')

	if flagChar == '0' {
		return false, nil
	} else if flagChar == '1' {
		return true, nil
	} else {
		return false, errors.New("invalid flag value")
	}
}

// hasMoreData checks if there is more data to parse.
func (s *SvgPathStringSource) hasMoreData() bool {
	return s.idx < s.length
}

// parseSegment parses a segment from the string.
func (s *SvgPathStringSource) parseSegment() (PathSegmentData, error) {
	if !s.hasMoreData() {
		return PathSegmentData{}, errors.New("no more data")
	}

	var segment PathSegmentData
	lookahead := rune(s.str[s.idx])
	command := mapLetterToSegmentType(lookahead)

	if s.previousCommand == SvgPathSegTypeUnknown {
		if command != SvgPathSegTypeMoveToRel && command != SvgPathSegTypeMoveToAbs {
			return PathSegmentData{}, errors.New("expected to find moveTo command")
		}
		s.idx++
	} else if command == SvgPathSegTypeUnknown {
		command = s.maybeImplicitCommand(lookahead, command)
		if command == SvgPathSegTypeUnknown {
			return PathSegmentData{}, errors.New("expected a path command")
		}
	} else {
		s.idx++
	}

	segment.Command = command
	s.previousCommand = command

	switch segment.Command {
	case SvgPathSegTypeCubicToRel, SvgPathSegTypeCubicToAbs:
		x1, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y1, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.Point1 = PathOffset{x1, y1}
		fallthrough
	case SvgPathSegTypeSmoothCubicToRel, SvgPathSegTypeSmoothCubicToAbs:
		x2, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y2, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.Point2 = PathOffset{x2, y2}
		fallthrough
	case SvgPathSegTypeMoveToRel, SvgPathSegTypeMoveToAbs, SvgPathSegTypeLineToRel, SvgPathSegTypeLineToAbs, SvgPathSegTypeSmoothQuadToRel, SvgPathSegTypeSmoothQuadToAbs:
		x, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.TargetPoint = PathOffset{x, y}
	case SvgPathSegTypeLineToHorizontalRel, SvgPathSegTypeLineToHorizontalAbs:
		x, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.TargetPoint = PathOffset{x, segment.TargetPoint.Dy}
	case SvgPathSegTypeLineToVerticalRel, SvgPathSegTypeLineToVerticalAbs:
		y, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.TargetPoint = PathOffset{segment.TargetPoint.Dx, y}
	case SvgPathSegTypeClose:
		s.skipOptionalSvgSpaces()
	case SvgPathSegTypeQuadToRel, SvgPathSegTypeQuadToAbs:
		x1, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y1, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.Point1 = PathOffset{x1, y1}
		x, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.TargetPoint = PathOffset{x, y}
	case SvgPathSegTypeArcToRel, SvgPathSegTypeArcToAbs:
		x1, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y1, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.Point1 = PathOffset{x1, y1}
		angle, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.ArcAngle = angle
		large, err := s.parseArcFlag()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.ArcLarge = large
		sweep, err := s.parseArcFlag()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.ArcSweep = sweep
		x, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		y, err := s.parseNumber()
		if err != nil {
			return PathSegmentData{}, err
		}
		segment.TargetPoint = PathOffset{x, y}
	case SvgPathSegTypeUnknown:
		return PathSegmentData{}, errors.New("unknown segment command")
	}

	return segment, nil
}

// isValidRange checks if a number is within the valid range.
func isValidRange(x float64) bool {
	return x >= -math.MaxFloat64 && x <= math.MaxFloat64
}

// isValidExponent checks if an exponent is within the valid range.
func isValidExponent(x float64) bool {
	return x >= -37 && x <= 38
}

// mapLetterToSegmentType maps a letter to a segment type.
func mapLetterToSegmentType(c rune) SvgPathSegType {
	switch c {
	case 'M':
		return SvgPathSegTypeMoveToAbs
	case 'm':
		return SvgPathSegTypeMoveToRel
	case 'L':
		return SvgPathSegTypeLineToAbs
	case 'l':
		return SvgPathSegTypeLineToRel
	case 'H':
		return SvgPathSegTypeLineToHorizontalAbs
	case 'h':
		return SvgPathSegTypeLineToHorizontalRel
	case 'V':
		return SvgPathSegTypeLineToVerticalAbs
	case 'v':
		return SvgPathSegTypeLineToVerticalRel
	case 'C':
		return SvgPathSegTypeCubicToAbs
	case 'c':
		return SvgPathSegTypeCubicToRel
	case 'S':
		return SvgPathSegTypeSmoothCubicToAbs
	case 's':
		return SvgPathSegTypeSmoothCubicToRel
	case 'Q':
		return SvgPathSegTypeQuadToAbs
	case 'q':
		return SvgPathSegTypeQuadToRel
	case 'T':
		return SvgPathSegTypeSmoothQuadToAbs
	case 't':
		return SvgPathSegTypeSmoothQuadToRel
	case 'A':
		return SvgPathSegTypeArcToAbs
	case 'a':
		return SvgPathSegTypeArcToRel
	case 'Z', 'z':
		return SvgPathSegTypeClose
	default:
		return SvgPathSegTypeUnknown
	}
}

// SvgPathSegType represents the type of an SVG path segment.
type SvgPathSegType int

const (
	SvgPathSegTypeUnknown SvgPathSegType = iota
	SvgPathSegTypeMoveToAbs
	SvgPathSegTypeMoveToRel
	SvgPathSegTypeLineToAbs
	SvgPathSegTypeLineToRel
	SvgPathSegTypeLineToHorizontalAbs
	SvgPathSegTypeLineToHorizontalRel
	SvgPathSegTypeLineToVerticalAbs
	SvgPathSegTypeLineToVerticalRel
	SvgPathSegTypeCubicToAbs
	SvgPathSegTypeCubicToRel
	SvgPathSegTypeSmoothCubicToAbs
	SvgPathSegTypeSmoothCubicToRel
	SvgPathSegTypeQuadToAbs
	SvgPathSegTypeQuadToRel
	SvgPathSegTypeSmoothQuadToAbs
	SvgPathSegTypeSmoothQuadToRel
	SvgPathSegTypeArcToAbs
	SvgPathSegTypeArcToRel
	SvgPathSegTypeClose
)

// PathSegmentData represents a segment of an SVG path.
type PathSegmentData struct {
	Command     SvgPathSegType
	TargetPoint PathOffset
	Point1      PathOffset
	Point2      PathOffset
	ArcSweep    bool
	ArcLarge    bool
	ArcAngle    float64
}

// String returns a string representation of the PathSegmentData.
func (p PathSegmentData) String() string {
	return fmt.Sprintf("PathSegmentData{%v %v %v %v %v %v}", p.Command, p.TargetPoint, p.Point1, p.Point2, p.ArcSweep, p.ArcLarge)
}

// SvgPathNormalizer normalizes SVG path segments.
type SvgPathNormalizer struct {
	currentPoint PathOffset
	subPathPoint PathOffset
	controlPoint PathOffset
	lastCommand  SvgPathSegType
}

// NewSvgPathNormalizer creates a new SvgPathNormalizer.
func NewSvgPathNormalizer() *SvgPathNormalizer {
	return &SvgPathNormalizer{
		currentPoint: ZeroPathOffset(),
		subPathPoint: ZeroPathOffset(),
		controlPoint: ZeroPathOffset(),
		lastCommand:  SvgPathSegTypeUnknown,
	}
}

// emitSegment emits a normalized segment to the path.
func (n *SvgPathNormalizer) emitSegment(segment PathSegmentData, path PathProxy) {
	normSeg := segment
	switch segment.Command {
	case SvgPathSegTypeQuadToRel:
		normSeg.Point1 = normSeg.Point1.Add(n.currentPoint)
		normSeg.TargetPoint = normSeg.TargetPoint.Add(n.currentPoint)
	case SvgPathSegTypeCubicToRel:
		normSeg.Point1 = normSeg.Point1.Add(n.currentPoint)
		fallthrough
	case SvgPathSegTypeSmoothCubicToRel:
		normSeg.Point2 = normSeg.Point2.Add(n.currentPoint)
		fallthrough
	case SvgPathSegTypeMoveToRel, SvgPathSegTypeLineToRel, SvgPathSegTypeLineToHorizontalRel, SvgPathSegTypeLineToVerticalRel, SvgPathSegTypeSmoothQuadToRel, SvgPathSegTypeArcToRel:
		normSeg.TargetPoint = normSeg.TargetPoint.Add(n.currentPoint)
	case SvgPathSegTypeLineToHorizontalAbs:
		normSeg.TargetPoint = PathOffset{normSeg.TargetPoint.Dx, n.currentPoint.Dy}
	case SvgPathSegTypeLineToVerticalAbs:
		normSeg.TargetPoint = PathOffset{n.currentPoint.Dx, normSeg.TargetPoint.Dy}
	case SvgPathSegTypeClose:
		normSeg.TargetPoint = n.subPathPoint
	}

	switch segment.Command {
	case SvgPathSegTypeMoveToRel, SvgPathSegTypeMoveToAbs:
		n.subPathPoint = normSeg.TargetPoint
		path.MoveTo(normSeg.TargetPoint.Dx, normSeg.TargetPoint.Dy)
	case SvgPathSegTypeLineToRel, SvgPathSegTypeLineToAbs, SvgPathSegTypeLineToHorizontalRel, SvgPathSegTypeLineToHorizontalAbs, SvgPathSegTypeLineToVerticalRel, SvgPathSegTypeLineToVerticalAbs:
		path.LineTo(normSeg.TargetPoint.Dx, normSeg.TargetPoint.Dy)
	case SvgPathSegTypeClose:
		path.Close()
	case SvgPathSegTypeSmoothCubicToRel, SvgPathSegTypeSmoothCubicToAbs:
		if !n.isCubicCommand(n.lastCommand) {
			normSeg.Point1 = n.currentPoint
		} else {
			normSeg.Point1 = n.reflectedPoint(n.currentPoint, n.controlPoint)
		}
		fallthrough
	case SvgPathSegTypeCubicToRel, SvgPathSegTypeCubicToAbs:
		n.controlPoint = normSeg.Point2
		path.CubicTo(normSeg.Point1.Dx, normSeg.Point1.Dy, normSeg.Point2.Dx, normSeg.Point2.Dy, normSeg.TargetPoint.Dx, normSeg.TargetPoint.Dy)
	case SvgPathSegTypeSmoothQuadToRel, SvgPathSegTypeSmoothQuadToAbs:
		if !n.isQuadraticCommand(n.lastCommand) {
			normSeg.Point1 = n.currentPoint
		} else {
			normSeg.Point1 = n.reflectedPoint(n.currentPoint, n.controlPoint)
		}
		fallthrough
	case SvgPathSegTypeQuadToRel, SvgPathSegTypeQuadToAbs:
		n.controlPoint = normSeg.Point1
		normSeg.Point1 = n.blendPoints(n.currentPoint, n.controlPoint)
		normSeg.Point2 = n.blendPoints(normSeg.TargetPoint, n.controlPoint)
		path.CubicTo(normSeg.Point1.Dx, normSeg.Point1.Dy, normSeg.Point2.Dx, normSeg.Point2.Dy, normSeg.TargetPoint.Dx, normSeg.TargetPoint.Dy)
	case SvgPathSegTypeArcToRel, SvgPathSegTypeArcToAbs:
		if !n.decomposeArcToCubic(n.currentPoint, normSeg, path) {
			path.LineTo(normSeg.TargetPoint.Dx, normSeg.TargetPoint.Dy)
		}
	default:
		panic("invalid command type in path")
	}

	n.currentPoint = normSeg.TargetPoint

	if !n.isCubicCommand(segment.Command) && !n.isQuadraticCommand(segment.Command) {
		n.controlPoint = n.currentPoint
	}

	n.lastCommand = segment.Command
}

// isCubicCommand checks if a command is a cubic command.
func (n *SvgPathNormalizer) isCubicCommand(command SvgPathSegType) bool {
	return command == SvgPathSegTypeCubicToAbs || command == SvgPathSegTypeCubicToRel || command == SvgPathSegTypeSmoothCubicToAbs || command == SvgPathSegTypeSmoothCubicToRel
}

// isQuadraticCommand checks if a command is a quadratic command.
func (n *SvgPathNormalizer) isQuadraticCommand(command SvgPathSegType) bool {
	return command == SvgPathSegTypeQuadToAbs || command == SvgPathSegTypeQuadToRel || command == SvgPathSegTypeSmoothQuadToAbs || command == SvgPathSegTypeSmoothQuadToRel
}

// reflectedPoint returns the reflection of a point over another point.
func (n *SvgPathNormalizer) reflectedPoint(reflectedIn, pointToReflect PathOffset) PathOffset {
	return PathOffset{2*reflectedIn.Dx - pointToReflect.Dx, 2*reflectedIn.Dy - pointToReflect.Dy}
}

// blendPoints blends two points.
func (n *SvgPathNormalizer) blendPoints(p1, p2 PathOffset) PathOffset {
	return PathOffset{(p1.Dx + 2*p2.Dx) / 3, (p1.Dy + 2*p2.Dy) / 3}
}

// decomposeArcToCubic decomposes an arc segment into cubic segments.
func (n *SvgPathNormalizer) decomposeArcToCubic(currentPoint PathOffset, arcSegment PathSegmentData, path PathProxy) bool {
	rx := math.Abs(arcSegment.Point1.Dx)
	ry := math.Abs(arcSegment.Point1.Dy)
	if rx == 0 || ry == 0 {
		return false
	}

	if arcSegment.TargetPoint == currentPoint {
		return false
	}

	angle := math.Pi * arcSegment.ArcAngle / 180.0

	midPointDistance := currentPoint.Subtract(arcSegment.TargetPoint).Multiply(0.5)

	pointTransform := mgl32.HomogRotate3DZ(float32(-angle))

	transformedMidPoint := mapPoint(pointTransform, PathOffset{midPointDistance.Dx, midPointDistance.Dy})

	squareRx := rx * rx
	squareRy := ry * ry
	squareX := transformedMidPoint.Dx * transformedMidPoint.Dx
	squareY := transformedMidPoint.Dy * transformedMidPoint.Dy

	radiiScale := squareX/squareRx + squareY/squareRy
	if radiiScale > 1.0 {
		rx *= math.Sqrt(radiiScale)
		ry *= math.Sqrt(radiiScale)
	}

	pointTransform = mgl32.Scale3D(float32(1.0/rx), float32(1.0/ry), float32(1.0/rx)).Mul4(mgl32.HomogRotate3DZ(float32(-angle)))

	point1 := mapPoint(pointTransform, currentPoint)
	point2 := mapPoint(pointTransform, arcSegment.TargetPoint)
	delta := point2.Subtract(point1)

	d := delta.Dx*delta.Dx + delta.Dy*delta.Dy
	scaleFactorSquared := math.Max(1.0/d-0.25, 0.0)
	scaleFactor := math.Sqrt(scaleFactorSquared)
	if !isFinite(scaleFactor) {
		scaleFactor = 0.0
	}

	if arcSegment.ArcSweep == arcSegment.ArcLarge {
		scaleFactor = -scaleFactor
	}

	delta = delta.Multiply(scaleFactor)
	centerPoint := point1.Add(point2).Multiply(0.5).Translate(-delta.Dy, delta.Dx)

	theta1 := (point1.Subtract(centerPoint)).Direction()
	theta2 := (point2.Subtract(centerPoint)).Direction()

	thetaArc := theta2 - theta1

	if thetaArc < 0.0 && arcSegment.ArcSweep {
		thetaArc += 2 * math.Pi
	} else if thetaArc > 0.0 && !arcSegment.ArcSweep {
		thetaArc -= 2 * math.Pi
	}

	pointTransform = mgl32.HomogRotate3DZ(float32(angle)).Mul4(mgl32.Scale3D(float32(rx), float32(ry), float32(rx)))

	segments := int(math.Ceil(math.Abs(thetaArc) / (math.Pi/2 + 0.001)))
	for i := 0; i < segments; i++ {
		startTheta := theta1 + float64(i)*thetaArc/float64(segments)
		endTheta := theta1 + float64(i+1)*thetaArc/float64(segments)

		t := (8.0 / 6.0) * math.Tan(0.25*(endTheta-startTheta))
		if !isFinite(t) {
			return false
		}
		sinStartTheta := math.Sin(startTheta)
		cosStartTheta := math.Cos(startTheta)
		sinEndTheta := math.Sin(endTheta)
		cosEndTheta := math.Cos(endTheta)

		point1 := PathOffset{cosStartTheta - t*sinStartTheta, sinStartTheta + t*cosStartTheta}.Translate(centerPoint.Dx, centerPoint.Dy)
		targetPoint := PathOffset{cosEndTheta, sinEndTheta}.Translate(centerPoint.Dx, centerPoint.Dy)
		point2 := targetPoint.Translate(t*sinEndTheta, -t*cosEndTheta)

		cubicSegment := PathSegmentData{
			Command:     SvgPathSegTypeCubicToAbs,
			Point1:      mapPoint(pointTransform, point1),
			Point2:      mapPoint(pointTransform, point2),
			TargetPoint: mapPoint(pointTransform, targetPoint),
		}

		path.CubicTo(cubicSegment.Point1.Dx, cubicSegment.Point1.Dy, cubicSegment.Point2.Dx, cubicSegment.Point2.Dy, cubicSegment.TargetPoint.Dx, cubicSegment.TargetPoint.Dy)
	}
	return true
}

func isFinite(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f) && !math.IsInf(f, 1)
}
