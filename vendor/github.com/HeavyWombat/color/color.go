package color

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

var (
	// NoColor defines if the output is colorized or not. It's dynamically set to
	// false or true based on the stdout's file descriptor referring to a terminal
	// or not. This is a global option and affects all colors. For more control
	// over each color block use the methods DisableColor() individually.
	NoColor = os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()))

	// Output defines the standard output of the print functions. By default
	// os.Stdout is used.
	Output = colorable.NewColorableStdout()

	// colorsCache is used to reduce the count of created Color objects and
	// allows to reuse already created objects with required Attribute.
	colorsCache   = make(map[Attribute]*Color)
	colorsCacheMu sync.Mutex // protects colorsCache
)

// Color defines a custom color object which is defined by SGR parameters.
type Color struct {
	params  []Attribute
	noColor *bool
}

// Attribute defines a single SGR Code
type Attribute int

// String defines a string that consists of differently colored text segments
type String []Segment

// Segment defines a piece of text and its designed style attributes (which can be empty)
type Segment struct {
	attributes []Attribute
	data       string
}

// https://regex101.com/segmentRegexp/ulipXZ/3
var segmentRegexp = regexp.MustCompile(fmt.Sprintf(`(?m)(.*?)((%s\[(\d+(;\d+)*)m)(.+?)(%s\[0m))`, escape, escape))

const escape = "\x1b"

// Base attributes
const (
	Reset Attribute = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground text colors
const (
	FgBlack Attribute = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Foreground Hi-Intensity text colors
const (
	FgHiBlack Attribute = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background text colors
const (
	BgBlack Attribute = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
)

// Background Hi-Intensity text colors
const (
	BgHiBlack Attribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

// New returns a newly created color object.
func New(value ...Attribute) *Color {
	c := &Color{params: make([]Attribute, 0)}
	c.Add(value...)
	return c
}

// Set sets the given parameters immediately. It will change the color of
// output with the given SGR parameters until color.Unset() is called.
func Set(p ...Attribute) *Color {
	c := New(p...)
	c.Set()
	return c
}

// Unset resets all escape attributes and clears the output. Usually should
// be called after Set().
func Unset() {
	if NoColor {
		return
	}

	fmt.Fprintf(Output, "%s[%dm", escape, Reset)
}

func (c String) String() string {
	var buf bytes.Buffer
	for _, segment := range c {
		if len(segment.attributes) > 0 {
			buf.WriteString(New(segment.attributes...).Sprint(segment.data))
		} else {
			buf.WriteString(segment.data)
		}
	}

	return buf.String()
}

// Set sets the SGR sequence.
func (c *Color) Set() *Color {
	if c.isNoColorSet() {
		return c
	}

	fmt.Fprintf(Output, c.format())
	return c
}

func (c *Color) unset() {
	if c.isNoColorSet() {
		return
	}

	Unset()
}

func (c *Color) setWriter(w io.Writer) *Color {
	if c.isNoColorSet() {
		return c
	}

	fmt.Fprintf(w, c.format())
	return c
}

func (c *Color) unsetWriter(w io.Writer) {
	if c.isNoColorSet() {
		return
	}

	if NoColor {
		return
	}

	fmt.Fprintf(w, "%s[%dm", escape, Reset)
}

// Add is used to chain SGR parameters. Use as many as parameters to combine
// and create custom color objects. Example: Add(color.FgRed, color.Underline).
func (c *Color) Add(value ...Attribute) *Color {
	c.params = append(c.params, value...)
	return c
}

func (c *Color) prepend(value Attribute) {
	c.params = append(c.params, 0)
	copy(c.params[1:], c.params[0:])
	c.params[0] = value
}

// Fprint formats using the default formats for its operands and writes to w.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
// On Windows, users should wrap w with colorable.NewColorable() if w is of
// type *os.File.
func (c *Color) Fprint(w io.Writer, a ...interface{}) (n int, err error) {
	c.setWriter(w)
	defer c.unsetWriter(w)

	return fmt.Fprint(w, a...)
}

// Print formats using the default formats for its operands and writes to
// standard output. Spaces are added between operands when neither is a
// string. It returns the number of bytes written and any write error
// encountered. This is the standard fmt.Print() method wrapped with the given
// color.
func (c *Color) Print(a ...interface{}) (n int, err error) {
	c.Set()
	defer c.unset()

	return fmt.Fprint(Output, a...)
}

// Fprintf formats according to a format specifier and writes to w.
// It returns the number of bytes written and any write error encountered.
// On Windows, users should wrap w with colorable.NewColorable() if w is of
// type *os.File.
func (c *Color) Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	c.setWriter(w)
	defer c.unsetWriter(w)

	return fmt.Fprintf(w, format, a...)
}

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
// This is the standard fmt.Printf() method wrapped with the given color.
func (c *Color) Printf(format string, a ...interface{}) (n int, err error) {
	c.Set()
	defer c.unset()

	return fmt.Fprintf(Output, format, a...)
}

// Fprintln formats using the default formats for its operands and writes to w.
// Spaces are always added between operands and a newline is appended.
// On Windows, users should wrap w with colorable.NewColorable() if w is of
// type *os.File.
func (c *Color) Fprintln(w io.Writer, a ...interface{}) (n int, err error) {
	c.setWriter(w)
	defer c.unsetWriter(w)

	return fmt.Fprintln(w, a...)
}

// Println formats using the default formats for its operands and writes to
// standard output. Spaces are always added between operands and a newline is
// appended. It returns the number of bytes written and any write error
// encountered. This is the standard fmt.Print() method wrapped with the given
// color.
func (c *Color) Println(a ...interface{}) (n int, err error) {
	c.Set()
	defer c.unset()

	return fmt.Fprintln(Output, a...)
}

// Sprint is just like Print, but returns a string instead of printing it.
func (c *Color) Sprint(a ...interface{}) string {
	return c.wrap(fmt.Sprint(a...))
}

// Sprintln is just like Println, but returns a string instead of printing it.
func (c *Color) Sprintln(a ...interface{}) string {
	return c.wrap(fmt.Sprintln(a...))
}

// Sprintf is just like Printf, but returns a string instead of printing it.
func (c *Color) Sprintf(format string, a ...interface{}) string {
	return c.wrap(fmt.Sprintf(format, a...))
}

// FprintFunc returns a new function that prints the passed arguments as
// colorized with color.Fprint().
func (c *Color) FprintFunc() func(w io.Writer, a ...interface{}) {
	return func(w io.Writer, a ...interface{}) {
		c.Fprint(w, a...)
	}
}

// PrintFunc returns a new function that prints the passed arguments as
// colorized with color.Print().
func (c *Color) PrintFunc() func(a ...interface{}) {
	return func(a ...interface{}) {
		c.Print(a...)
	}
}

// FprintfFunc returns a new function that prints the passed arguments as
// colorized with color.Fprintf().
func (c *Color) FprintfFunc() func(w io.Writer, format string, a ...interface{}) {
	return func(w io.Writer, format string, a ...interface{}) {
		c.Fprintf(w, format, a...)
	}
}

// PrintfFunc returns a new function that prints the passed arguments as
// colorized with color.Printf().
func (c *Color) PrintfFunc() func(format string, a ...interface{}) {
	return func(format string, a ...interface{}) {
		c.Printf(format, a...)
	}
}

// FprintlnFunc returns a new function that prints the passed arguments as
// colorized with color.Fprintln().
func (c *Color) FprintlnFunc() func(w io.Writer, a ...interface{}) {
	return func(w io.Writer, a ...interface{}) {
		c.Fprintln(w, a...)
	}
}

// PrintlnFunc returns a new function that prints the passed arguments as
// colorized with color.Println().
func (c *Color) PrintlnFunc() func(a ...interface{}) {
	return func(a ...interface{}) {
		c.Println(a...)
	}
}

// SprintFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprint(). Useful to put into or mix into other
// string. Windows users should use this in conjunction with color.Output, example:
//
//	put := New(FgYellow).SprintFunc()
//	fmt.Fprintf(color.Output, "This is a %s", put("warning"))
func (c *Color) SprintFunc() func(a ...interface{}) string {
	return func(a ...interface{}) string {
		return c.wrap(fmt.Sprint(a...))
	}
}

// SprintfFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprintf(). Useful to put into or mix into other
// string. Windows users should use this in conjunction with color.Output.
func (c *Color) SprintfFunc() func(format string, a ...interface{}) string {
	return func(format string, a ...interface{}) string {
		return c.wrap(fmt.Sprintf(format, a...))
	}
}

// SprintlnFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprintln(). Useful to put into or mix into other
// string. Windows users should use this in conjunction with color.Output.
func (c *Color) SprintlnFunc() func(a ...interface{}) string {
	return func(a ...interface{}) string {
		return c.wrap(fmt.Sprintln(a...))
	}
}

// sequence returns a formatted SGR sequence to be plugged into a "\x1b[...m"
// an example output might be: "1;36" -> bold cyan
func (c *Color) sequence() string {
	format := make([]string, len(c.params))
	for i, v := range c.params {
		format[i] = strconv.Itoa(int(v))
	}

	return strings.Join(format, ";")
}

// wrap wraps the s string with the colors attributes. The string is ready to
// be printed.
func (c *Color) wrap(s string) string {
	if c.isNoColorSet() {
		return s
	}

	cstring, err := breakUpStringIntoColorSegments(s)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	for _, segment := range cstring {
		attributes := mergeAttributes(c.params, segment.attributes)
		format := make([]string, len(attributes))
		for i, attr := range attributes {
			format[i] = strconv.Itoa(int(attr))
		}

		sort.Strings(format)

		buf.WriteString(fmt.Sprintf("%s[%sm", escape, strings.Join(format, ";")))
		buf.WriteString(segment.data)
		buf.WriteString(fmt.Sprintf("%s[%dm", escape, Reset))
	}

	return buf.String()
}

func (c *Color) format() string {
	return fmt.Sprintf("%s[%sm", escape, c.sequence())
}

func (c *Color) unformat() string {
	return fmt.Sprintf("%s[%dm", escape, Reset)
}

// DisableColor disables the color output. Useful to not change any existing
// code and still being able to output. Can be used for flags like
// "--no-color". To enable back use EnableColor() method.
func (c *Color) DisableColor() {
	c.noColor = boolPtr(true)
}

// EnableColor enables the color output. Use it in conjunction with
// DisableColor(). Otherwise this method has no side effects.
func (c *Color) EnableColor() {
	c.noColor = boolPtr(false)
}

func (c *Color) isNoColorSet() bool {
	// check first if we have user setted action
	if c.noColor != nil {
		return *c.noColor
	}

	// if not return the global option, which is disabled by default
	return NoColor
}

// Equals returns a boolean value indicating whether two colors are equal.
func (c *Color) Equals(c2 *Color) bool {
	if len(c.params) != len(c2.params) {
		return false
	}

	for _, attr := range c.params {
		if !c2.attrExists(attr) {
			return false
		}
	}

	return true
}

func (c *Color) attrExists(a Attribute) bool {
	for _, attr := range c.params {
		if attr == a {
			return true
		}
	}

	return false
}

// RemoveAllEscapeSequences return the input string with all escape sequences removed
func RemoveAllEscapeSequences(input string) string {
	regexp := regexp.MustCompile(escape + `\[\d+(;\d+)*m`)

	for loc := regexp.FindStringIndex(input); loc != nil; loc = regexp.FindStringIndex(input) {
		start := loc[0]
		end := loc[1]
		input = strings.Replace(input, input[start:end], "", -1)
	}

	return input
}

func breakUpStringIntoColorSegments(input string) (String, error) {
	var result String

	lastPos := 0
	for _, submatch := range segmentRegexp.FindAllStringSubmatchIndex(input, -1) {
		// 0:1 full capture group
		// 2:3 text before first espace sequence
		// 4:5 text in espace sequence including the start and end escape sequence
		// 6:7 start escape sequence
		// 8:9 attributes of start escape sequence
		// 10:11 optinal attributes of start escape sequence
		// 12:13 the actual text
		// 14:15 end escape sequence
		headTextSegment := input[submatch[2]:submatch[3]]
		attributesPart := input[submatch[8]:submatch[9]]
		textSegment := input[submatch[12]:submatch[13]]

		// Break up the style attributes to convert it from string into proper Attributes
		splitted := strings.Split(attributesPart, ";")
		attributes := make([]Attribute, 0, len(splitted))
		for _, attribute := range splitted {
			var value int
			var err error
			if value, err = strconv.Atoi(attribute); err != nil {
				return nil, fmt.Errorf("Failed to split input string in its colored segments: %s", err.Error())
			}

			attributes = append(attributes, Attribute(value))
		}

		// Append any piece of text that is between two identified segments (mostely line breaks and stuff)
		if lastPos != submatch[0] {
			result = append(result, Segment{data: input[lastPos:submatch[0]]})
		}

		// Append text segment that was identified as being before the styled segment
		result = append(result, Segment{data: headTextSegment})

		// Append the segment including its identified style attributes
		result = append(result, Segment{data: textSegment, attributes: attributes})

		lastPos = submatch[1]
	}

	// Append any trailing text that is still left
	if lastPos < len(input) {
		result = append(result, Segment{data: input[lastPos:]})
	}

	return result, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func getCachedColor(p Attribute) *Color {
	colorsCacheMu.Lock()
	defer colorsCacheMu.Unlock()

	c, ok := colorsCache[p]
	if !ok {
		c = New(p)
		colorsCache[p] = c
	}

	return c
}

func colorPrint(format string, p Attribute, a ...interface{}) {
	c := getCachedColor(p)

	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	if len(a) == 0 {
		c.Print(format)
	} else {
		c.Printf(format, a...)
	}
}

func colorString(format string, p Attribute, a ...interface{}) string {
	c := getCachedColor(p)

	if len(a) == 0 {
		return c.SprintFunc()(format)
	}

	return c.SprintfFunc()(format, a...)
}

func mergeAttributes(set1, set2 []Attribute) []Attribute {
	// TODO Set-up rules which style takes precendence over which other style

	tmp := make(map[Attribute]struct{}, 0)

	for _, attr := range set1 {
		tmp[attr] = struct{}{}
	}

	for _, attr := range set2 {
		tmp[attr] = struct{}{}
	}

	result := make([]Attribute, 0, len(tmp))
	for attr := range tmp {
		result = append(result, attr)
	}

	return result
}

// Black is a convenient helper function to print with black foreground. A
// newline is appended to format by default.
func Black(format string, a ...interface{}) { colorPrint(format, FgBlack, a...) }

// Red is a convenient helper function to print with red foreground. A
// newline is appended to format by default.
func Red(format string, a ...interface{}) { colorPrint(format, FgRed, a...) }

// Green is a convenient helper function to print with green foreground. A
// newline is appended to format by default.
func Green(format string, a ...interface{}) { colorPrint(format, FgGreen, a...) }

// Yellow is a convenient helper function to print with yellow foreground.
// A newline is appended to format by default.
func Yellow(format string, a ...interface{}) { colorPrint(format, FgYellow, a...) }

// Blue is a convenient helper function to print with blue foreground. A
// newline is appended to format by default.
func Blue(format string, a ...interface{}) { colorPrint(format, FgBlue, a...) }

// Magenta is a convenient helper function to print with magenta foreground.
// A newline is appended to format by default.
func Magenta(format string, a ...interface{}) { colorPrint(format, FgMagenta, a...) }

// Cyan is a convenient helper function to print with cyan foreground. A
// newline is appended to format by default.
func Cyan(format string, a ...interface{}) { colorPrint(format, FgCyan, a...) }

// White is a convenient helper function to print with white foreground. A
// newline is appended to format by default.
func White(format string, a ...interface{}) { colorPrint(format, FgWhite, a...) }

// BlackString is a convenient helper function to return a string with black
// foreground.
func BlackString(format string, a ...interface{}) string { return colorString(format, FgBlack, a...) }

// RedString is a convenient helper function to return a string with red
// foreground.
func RedString(format string, a ...interface{}) string { return colorString(format, FgRed, a...) }

// GreenString is a convenient helper function to return a string with green
// foreground.
func GreenString(format string, a ...interface{}) string { return colorString(format, FgGreen, a...) }

// YellowString is a convenient helper function to return a string with yellow
// foreground.
func YellowString(format string, a ...interface{}) string { return colorString(format, FgYellow, a...) }

// BlueString is a convenient helper function to return a string with blue
// foreground.
func BlueString(format string, a ...interface{}) string { return colorString(format, FgBlue, a...) }

// MagentaString is a convenient helper function to return a string with magenta
// foreground.
func MagentaString(format string, a ...interface{}) string {
	return colorString(format, FgMagenta, a...)
}

// CyanString is a convenient helper function to return a string with cyan
// foreground.
func CyanString(format string, a ...interface{}) string { return colorString(format, FgCyan, a...) }

// WhiteString is a convenient helper function to return a string with white
// foreground.
func WhiteString(format string, a ...interface{}) string { return colorString(format, FgWhite, a...) }

// HiBlack is a convenient helper function to print with hi-intensity black foreground. A
// newline is appended to format by default.
func HiBlack(format string, a ...interface{}) { colorPrint(format, FgHiBlack, a...) }

// HiRed is a convenient helper function to print with hi-intensity red foreground. A
// newline is appended to format by default.
func HiRed(format string, a ...interface{}) { colorPrint(format, FgHiRed, a...) }

// HiGreen is a convenient helper function to print with hi-intensity green foreground. A
// newline is appended to format by default.
func HiGreen(format string, a ...interface{}) { colorPrint(format, FgHiGreen, a...) }

// HiYellow is a convenient helper function to print with hi-intensity yellow foreground.
// A newline is appended to format by default.
func HiYellow(format string, a ...interface{}) { colorPrint(format, FgHiYellow, a...) }

// HiBlue is a convenient helper function to print with hi-intensity blue foreground. A
// newline is appended to format by default.
func HiBlue(format string, a ...interface{}) { colorPrint(format, FgHiBlue, a...) }

// HiMagenta is a convenient helper function to print with hi-intensity magenta foreground.
// A newline is appended to format by default.
func HiMagenta(format string, a ...interface{}) { colorPrint(format, FgHiMagenta, a...) }

// HiCyan is a convenient helper function to print with hi-intensity cyan foreground. A
// newline is appended to format by default.
func HiCyan(format string, a ...interface{}) { colorPrint(format, FgHiCyan, a...) }

// HiWhite is a convenient helper function to print with hi-intensity white foreground. A
// newline is appended to format by default.
func HiWhite(format string, a ...interface{}) { colorPrint(format, FgHiWhite, a...) }

// HiBlackString is a convenient helper function to return a string with hi-intensity black
// foreground.
func HiBlackString(format string, a ...interface{}) string {
	return colorString(format, FgHiBlack, a...)
}

// HiRedString is a convenient helper function to return a string with hi-intensity red
// foreground.
func HiRedString(format string, a ...interface{}) string { return colorString(format, FgHiRed, a...) }

// HiGreenString is a convenient helper function to return a string with hi-intensity green
// foreground.
func HiGreenString(format string, a ...interface{}) string {
	return colorString(format, FgHiGreen, a...)
}

// HiYellowString is a convenient helper function to return a string with hi-intensity yellow
// foreground.
func HiYellowString(format string, a ...interface{}) string {
	return colorString(format, FgHiYellow, a...)
}

// HiBlueString is a convenient helper function to return a string with hi-intensity blue
// foreground.
func HiBlueString(format string, a ...interface{}) string { return colorString(format, FgHiBlue, a...) }

// HiMagentaString is a convenient helper function to return a string with hi-intensity magenta
// foreground.
func HiMagentaString(format string, a ...interface{}) string {
	return colorString(format, FgHiMagenta, a...)
}

// HiCyanString is a convenient helper function to return a string with hi-intensity cyan
// foreground.
func HiCyanString(format string, a ...interface{}) string { return colorString(format, FgHiCyan, a...) }

// HiWhiteString is a convenient helper function to return a string with hi-intensity white
// foreground.
func HiWhiteString(format string, a ...interface{}) string {
	return colorString(format, FgHiWhite, a...)
}
