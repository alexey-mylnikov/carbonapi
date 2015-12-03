// +build cairo

package main

import (
	"bufio"
	"bytes"
	"container/list"
	"fmt"
	"image/color"
	"math"
	"net/http"
	"strconv"
	"strings"

	cairo "github.com/martine/gocairo/cairo"
)

type HAlign int

const (
	H_ALIGN_LEFT   HAlign = 1
	H_ALIGN_CENTER HAlign = 2
	H_ALIGN_RIGHT  HAlign = 4
)

type VAlign int

const (
	V_ALIGN_TOP      VAlign = 8
	V_ALIGN_CENTER   VAlign = 16
	V_ALIGN_BOTTOM   VAlign = 32
	V_ALIGN_BASELINE VAlign = 64
)

type LineMode int

const (
	LineModeSlope     LineMode = 1
	LineModeStaircase LineMode = 2
	LineModeConnected LineMode = 4
)

type AreaMode int

const (
	AreaModeNone    = 1
	AreaModeFirst   = 2
	AreaModeAll     = 4
	AreaModeStacked = 8
)

type PieMode int

const (
	PieModeMaximum = 1
	PieModeMinimum = 2
	PieModeAverage = 4
)

type YAxisSide int

const (
	YAxisSideRight = 1
	YAxisSideLeft  = 2
)

/*
type FetchResponse struct {
        Name             *string   `protobuf:"bytes,1,req,name=name" json:"name,omitempty"`
        StartTime        *int32    `protobuf:"varint,2,req,name=startTime" json:"startTime,omitempty"`
        StopTime         *int32    `protobuf:"varint,3,req,name=stopTime" json:"stopTime,omitempty"`
        StepTime         *int32    `protobuf:"varint,4,req,name=stepTime" json:"stepTime,omitempty"`
        Values           []float64 `protobuf:"fixed64,5,rep,name=values" json:"values,omitempty"`
        IsAbsent         []bool    `protobuf:"varint,6,rep,name=isAbsent" json:"isAbsent,omitempty"`
        XXX_unrecognized []byte    `json:"-"`
}
*/

var customizable = [...]string{
	"width",
	"height",
	"margin",
	"bgcolor",
	"fgcolor",
	"fontName",
	"fontSize",
	"fontBold",
	"fontItalic",
	"colorList",
	"template",
	"yAxisSide",
	"outputFormat",
}

var unitSystems = map[string]map[string]uint64{
	"binary": {
		"Pi": 1125899906842624, // 1024^5
		"Ti": 1099511627776,    // 1024^4
		"Gi": 1073741824,       // 1024^3
		"Mi": 1048576,          // 1024^2
		"Ki": 1024,
	},
	"si": {
		"P": 1000000000000000, // 1000^5
		"T": 1000000000000,    // 1000^4
		"G": 1000000000,       // 1000^3
		"M": 1000000,          // 1000^2
		"K": 1000,
	},
}

// TODO: Current colors are not perfect match with graphite-api.
// TODO: Migrate to custom type
type cairoColor struct {
	r float64 // 0.0 .. 1.0
	g float64 // 0.0 .. 1.0
	b float64 // 0.0 .. 1.0
	a float64
}

type xAxisStruct struct {
	seconds       float32
	minorGridUnit uint32
	minorGridStep float32
	majorGridUnit uint32
	majorGridStep float32
	labelUnit     uint32
	labelStep     float32
	format        string
	maxInterval   uint32
}

var xAxisConfigs = []xAxisStruct{
	xAxisStruct{
		seconds:       0.00,
		minorGridUnit: 1, // SEC
		minorGridStep: 5,
		majorGridUnit: 60, // MIN
		majorGridStep: 1,
		labelUnit:     1, // SEC
		labelStep:     5,
		format:        "%H:%M:%S",
		maxInterval:   10 * 60, // 10 * MIN
	},
	xAxisStruct{
		seconds:       0.07,
		minorGridUnit: 1, // SEC
		minorGridStep: 10,
		majorGridUnit: 60, // MIN
		majorGridStep: 1,
		labelUnit:     1, // SEC
		labelStep:     10,
		format:        "%H:%M:%S",
		maxInterval:   20 * 60, // 10 * MIN
	},
	xAxisStruct{
		seconds:       0.14,
		minorGridUnit: 1, // SEC
		minorGridStep: 15,
		majorGridUnit: 60, // MIN
		majorGridStep: 1,
		labelUnit:     1, // SEC
		labelStep:     15,
		format:        "%H:%M:%S",
		maxInterval:   30 * 60, // 30 * MIN
	},
	xAxisStruct{
		seconds:       0.27,
		minorGridUnit: 1, // SEC
		minorGridStep: 30,
		majorGridUnit: 60, // MIN
		majorGridStep: 2,
		labelUnit:     60, // MIN
		labelStep:     1,
		format:        "%H:%M",
		maxInterval:   2 * 60 * 60, // 2 * HOUR
	},
	xAxisStruct{
		seconds:       0.5,
		minorGridUnit: 60, // MIN
		minorGridStep: 1,
		majorGridUnit: 60, // MIN
		majorGridStep: 2,
		labelUnit:     60, // MIN
		labelStep:     1,
		format:        "%H:%M",
		maxInterval:   2 * 60 * 60, // 2 * HOUR
	},
	xAxisStruct{
		seconds:       1.2,
		minorGridUnit: 60, // MIN
		minorGridStep: 1,
		majorGridUnit: 60, // MIN
		majorGridStep: 4,
		labelUnit:     60, // MIN
		labelStep:     2,
		format:        "%H:%M",
		maxInterval:   3 * 60 * 60, // 2 * HOUR
	},
	xAxisStruct{
		seconds:       2,
		minorGridUnit: 60, // MIN
		minorGridStep: 1,
		majorGridUnit: 60, // MIN
		majorGridStep: 10,
		labelUnit:     60, // MIN
		labelStep:     5,
		format:        "%H:%M",
		maxInterval:   6 * 60 * 60, // 2 * HOUR
	},
	xAxisStruct{
		seconds:       5,
		minorGridUnit: 60, // MIN
		minorGridStep: 2,
		majorGridUnit: 60, // MIN
		majorGridStep: 10,
		labelUnit:     60, // MIN
		labelStep:     10,
		format:        "%H:%M",
		maxInterval:   12 * 60 * 60, // 2 * HOUR
	},
	xAxisStruct{
		seconds:       10,
		minorGridUnit: 60, // MIN
		minorGridStep: 5,
		majorGridUnit: 60, // MIN
		majorGridStep: 20,
		labelUnit:     60, // MIN
		labelStep:     20,
		format:        "%H:%M",
		maxInterval:   1 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       30,
		minorGridUnit: 60, // MIN
		minorGridStep: 10,
		majorGridUnit: 60 * 60, // HOUR
		majorGridStep: 1,
		labelUnit:     60 * 60, // HOUR
		labelStep:     1,
		format:        "%H:%M",
		maxInterval:   2 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       60,
		minorGridUnit: 60, // MIN
		minorGridStep: 30,
		majorGridUnit: 60 * 60, // HOUR
		majorGridStep: 2,
		labelUnit:     60 * 60, // HOUR
		labelStep:     2,
		format:        "%H:%M",
		maxInterval:   2 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       100,
		minorGridUnit: 60 * 60, // HOUR
		minorGridStep: 2,
		majorGridUnit: 60 * 60, // HOUR
		majorGridStep: 4,
		labelUnit:     60 * 60, // HOUR
		labelStep:     4,
		format:        "%a %l%p",
		maxInterval:   2 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       255,
		minorGridUnit: 60 * 60, // HOUR
		minorGridStep: 6,
		majorGridUnit: 60 * 60, // HOUR
		majorGridStep: 12,
		labelUnit:     60 * 60, // HOUR
		labelStep:     12,
		format:        "%a %l%p",
		maxInterval:   10 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       600,
		minorGridUnit: 60 * 60, // HOUR
		minorGridStep: 6,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 1,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     1,
		format:        "%m/%d",
		maxInterval:   14 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       1200,
		minorGridUnit: 60 * 60, // HOUR
		minorGridStep: 12,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 1,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     1,
		format:        "%m/%d",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       2000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 1,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 2,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     2,
		format:        "%m/%d",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       4000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 2,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 4,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     4,
		format:        "%m/%d",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       8000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 3.5,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 7,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     7,
		format:        "%m/%d",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       16000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 7,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 14,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     14,
		format:        "%m/%d",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       32000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 15,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 30,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     30,
		format:        "%m/%d",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       64000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 30,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 60,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     60,
		format:        "%m/%d %Y",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       100000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 60,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 120,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     120,
		format:        "%m/%d %Y",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
	xAxisStruct{
		seconds:       120000,
		minorGridUnit: 24 * 60 * 60, // HOUR
		minorGridStep: 120,
		majorGridUnit: 24 * 60 * 60, // DAY
		majorGridStep: 240,
		labelUnit:     24 * 60 * 60, // DAY
		labelStep:     240,
		format:        "%m/%d %Y",
		maxInterval:   365 * 24 * 60 * 60, // 1 * DAY
	},
}

func getFloat32(s string, def float32) float32 {
	if s == "" {
		return def
	}

	n, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return def
	}

	return float32(n)
}

func getInt(s string, def int) int {
	if s == "" {
		return def
	}

	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return def
	}

	return int(n)
}

func getFontItalic(s string, def cairo.FontSlant) cairo.FontSlant {
	if def != cairo.FontSlantNormal && def != cairo.FontSlantItalic {
		panic("invalid default font Italic specified!!!!")
		// return cairo.FontSlantNormal
	}

	if s == "" {
		return def
	}

	switch s {
	case "True", "true", "1":
		return cairo.FontSlantItalic
	case "False", "false", "0":
		return cairo.FontSlantNormal
	}

	return def
}

func getFontWeight(s string, def cairo.FontWeight) cairo.FontWeight {
	if def != cairo.FontWeightBold && def != cairo.FontWeightNormal {
		panic("invalid default font Weight specified!!!!")
		// return cairo.FontWeightNormal
	}

	if s == "" {
		return def
	}

	switch s {
	case "True", "true", "1":
		return cairo.FontWeightBold
	case "False", "false", "0":
		return cairo.FontWeightNormal
	}

	return def
}

func getLineMode(s string, def LineMode) LineMode {
	if s == "" {
		return def
	}

	if s == "slope" {
		return LineModeSlope
	}
	if s == "staircase" {
		return LineModeStaircase
	}

	return LineModeConnected
}

func getAreaMode(s string, def AreaMode) AreaMode {
	if s == "" {
		return def
	}

	if s == "first" {
		return AreaModeFirst
	}
	if s == "all" {
		return AreaModeAll
	}
	if s == "stacked" {
		return AreaModeStacked
	}

	return AreaModeNone
}

func getPieMode(s string, def PieMode) PieMode {
	if s == "" {
		return def
	}

	if s == "maximum" {
		return PieModeMaximum
	}
	if s == "minimum" {
		return PieModeMinimum
	}

	return PieModeAverage
}

func getAxisSide(s string, def YAxisSide) YAxisSide {
	if s == "" {
		return def
	}

	if s == "right" {
		return YAxisSideRight
	}

	return YAxisSideLeft
}

type Area struct {
	xmin float64
	xmax float64
	ymin float64
	ymax float64
}

type Params struct {
	width      float64
	height     float64
	margin     int
	logBase    float32
	fgColor    color.RGBA
	bgColor    color.RGBA
	majorLine  color.RGBA
	minorLine  color.RGBA
	fontName   string
	fontSize   float64
	fontBold   cairo.FontWeight
	fontItalic cairo.FontSlant

	graphOnly   bool
	hideLegend  bool
	hideGrid    bool
	hideAxes    bool
	hideYAxis   bool
	yAxisSide   YAxisSide
	title       string
	vtitle      string
	vtitleRight string
	tz          string

	lineMode       LineMode
	areaMode       AreaMode
	pieMode        PieMode
	lineColors     []string
	lineWidth      float64
	connectedLimit float64

	yMin  float64
	yMax  float64
	xMin  float64
	xMax  float64
	yStep float64
	xStep float64

	yTop         float64
	yBottom      float64
	ySpan        float64
	graphHeight  float64
	yScaleFactor float64

	rightWidth  float64
	rightDashed bool
	rightColor  string
	leftWidth   float64
	leftDashed  bool
	leftColor   string

	dashed bool

	area        Area
	isPng       bool // TODO: png and svg use the same code
	fontExtents cairo.FontExtents

	uniqueLegend   bool
	secondYAxis    bool
	drawNullAsZero bool
	drawAsInfinite bool
}

func bool2int(b bool) int {
	if b {
		return 0
	} else {
		return 1
	}
}

type cairoSurfaceContext struct {
	context *cairo.Context
	surface *cairo.ImageSurface
}

func marshalPNGCairo(r *http.Request, results []*metricData) []byte {
	var params = Params{
		width:          getFloat64(r.FormValue("width"), 600),
		height:         getFloat64(r.FormValue("height"), 300),
		margin:         getInt(r.FormValue("margin"), 10),
		logBase:        getFloat32(r.FormValue("logBase"), 1.0),
		fgColor:        string2RGBA(getString(r.FormValue("fgcolor"), "black")),
		bgColor:        string2RGBA(getString(r.FormValue("bgcolor"), "white")),
		majorLine:      string2RGBA(getString(r.FormValue("majorLine"), "rose")),
		minorLine:      string2RGBA(getString(r.FormValue("minorLine"), "grey")),
		fontName:       getString(r.FormValue("fontName"), "Sans"),
		fontSize:       getFloat64(r.FormValue("fontSize"), 10.0),
		fontBold:       getFontWeight(r.FormValue("fontBold"), cairo.FontWeightNormal),
		fontItalic:     getFontItalic(r.FormValue("fontItalic"), cairo.FontSlantNormal),
		graphOnly:      getBool(r.FormValue("graphOnly"), false),
		hideLegend:     getBool(r.FormValue("hideLegend"), false),
		hideGrid:       getBool(r.FormValue("hideLegend"), false),
		hideAxes:       getBool(r.FormValue("hideLegend"), false),
		hideYAxis:      getBool(r.FormValue("hideLegend"), false),
		yAxisSide:      getAxisSide(r.FormValue("yAxisSide"), YAxisSideLeft),
		connectedLimit: getFloat64(r.FormValue("connectedLimit"), math.Inf(1)),
		lineMode:       getLineMode(r.FormValue("lineMode"), LineModeSlope),
		areaMode:       getAreaMode(r.FormValue("areaMode"), AreaModeNone),
		pieMode:        getPieMode(r.FormValue("pieMode"), PieModeAverage),
		lineWidth:      getFloat64(r.FormValue("lineWidth"), 1.2),

		dashed:      getBool(r.FormValue("dashed"), false),
		rightWidth:  getFloat64(r.FormValue("rightWidth"), 1.2),
		rightDashed: getBool(r.FormValue("rightDashed"), false),
		rightColor:  getString(r.FormValue("rightColor"), ""),

		leftWidth:  getFloat64(r.FormValue("leftWidth"), 1.2),
		leftDashed: getBool(r.FormValue("leftDashed"), false),
		leftColor:  getString(r.FormValue("leftColor"), ""),

		title:       getString(r.FormValue("title"), ""),
		vtitle:      getString(r.FormValue("vtitle"), ""),
		vtitleRight: getString(r.FormValue("title"), ""),

		lineColors: []string{"blue", "green", "red", "purple", "brown", "yellow", "aqua", "grey", "magenta", "pink", "gold", "rose"},
		isPng:      true,

		uniqueLegend:   getBool(r.FormValue("uniqueLegend"), false),
		drawNullAsZero: getBool(r.FormValue("drawNullAsZero"), false),
		drawAsInfinite: getBool(r.FormValue("drawAsInfinite"), false),
		yMin:           getFloat64(r.FormValue("yMin"), math.NaN()),
		yMax:           getFloat64(r.FormValue("yMax"), math.NaN()),
		yStep:          getFloat64(r.FormValue("yStep"), math.NaN()),
		xMin:           getFloat64(r.FormValue("xMin"), math.NaN()),
		xMax:           getFloat64(r.FormValue("xMax"), math.NaN()),
		xStep:          getFloat64(r.FormValue("xStep"), math.NaN()),
	}

	fmt.Printf("")
	margin := float64(params.margin)
	params.area.xmin = margin + 10
	params.area.xmax = params.width - margin
	params.area.ymin = margin
	params.area.ymax = params.height - margin
	params.hideLegend = getBool(r.FormValue("hideLegend"), len(results) > 10)

	var cr cairoSurfaceContext
	cr.surface = cairo.ImageSurfaceCreate(cairo.FormatARGB32, int(params.width), int(params.height))
	cr.context = cairo.Create(cr.surface.Surface)

	// Setting font parameters
	/*
		fontOpts := cairo.FontOptionsCreate()
		cr.context.GetFontOptions(fontOpts)
		fontOpts.SetAntialias(cairo.AntialiasGray)
		cr.context.SetFontOptions(fontOpts)
	*/

	setColor(&cr, &params.bgColor)
	drawRectangle(&cr, &params, 0, 0, params.width, params.height, true)

	drawGraph(&cr, &params, results)

	cr.surface.Flush()

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	cr.surface.WriteToPNG(writer)
	cr.surface.Finish()
	writer.Flush()

	return b.Bytes()
}

func drawGraph(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	var startTime, endTime, timeRange, tmp, minNumberOfPoints, maxNumberOfPoints int32
	left := list.New()
	right := list.New()
	params.secondYAxis = false

	startTime = -1
	endTime = -1
	minNumberOfPoints = -1
	maxNumberOfPoints = -1
	for _, res := range results {
		tmp = res.GetStartTime()
		if startTime == -1 || startTime > tmp {
			startTime = tmp
		}
		tmp = res.GetStopTime()
		if endTime == -1 || endTime > tmp {
			endTime = tmp
		}

		tmp = int32(len(res.Values))
		if minNumberOfPoints == -1 || tmp < minNumberOfPoints {
			minNumberOfPoints = tmp
		}
		if maxNumberOfPoints == -1 || tmp > maxNumberOfPoints {
			maxNumberOfPoints = tmp
		}

	}
	timeRange = endTime - startTime

	if timeRange <= 0 {
		x := params.width / 2.0
		y := params.height / 2.0
		setColor(cr, string2RGBAptr("red"))
		fontSize := math.Log(params.width * params.height)
		setFont(cr, params, fontSize)
		drawText(cr, params, "No Data", x, y, H_ALIGN_CENTER, V_ALIGN_TOP, 0)

		return
	}

	for _, res := range results {
		if res.secondYAxis {
			right.PushBack(res)
		} else {
			left.PushBack(res)
		}
	}

	if right.Len() > 0 {
		params.secondYAxis = true
		params.yAxisSide = YAxisSideLeft
	}

	if params.graphOnly {
		params.hideLegend = true
		params.hideGrid = true
		params.hideAxes = true
		params.hideYAxis = true
	}

	if params.yAxisSide == YAxisSideRight {
		params.margin = int(params.width)
	}

	if params.lineMode == LineModeSlope && minNumberOfPoints == 1 {
		params.lineMode = LineModeStaircase
	}

	var colorsCur, lineColorsLen int
	colorsCur = 0
	lineColorsLen = len(params.lineColors)
	for _, res := range results {
		if params.secondYAxis && res.secondYAxis {
			res.lineWidth = params.rightWidth
			res.dashed = params.rightDashed
			res.color = params.rightColor
		} else if params.secondYAxis {
			res.lineWidth = params.leftWidth
			res.dashed = params.leftDashed
			res.color = params.leftColor
		}
		if res.color == "" {
			res.color = params.lineColors[colorsCur]
			colorsCur += 1
			if colorsCur >= lineColorsLen {
				colorsCur = 0
			}
		}
	}

	if params.title != "" || params.vtitle != "" {
		titleSize := params.fontSize + math.Floor(math.Log(params.fontSize))

		setColor(cr, &params.fgColor)
		setFont(cr, params, titleSize)
	}

	if params.title != "" {
		drawTitle(cr, params)
	}
	if params.vtitle != "" {
		drawVTitle(cr, params, false)
	}
	if params.secondYAxis && params.vtitleRight != "" {
		drawVTitle(cr, params, true)
	}

	setFont(cr, params, params.fontSize)
	if !params.hideLegend {
		drawLegend(cr, params, results)
	}

	// Setup axes, labels and grid
	// First we adjust the drawing area size to fit X-axis labels
	if !params.hideAxes {
		params.area.ymax -= params.fontExtents.Ascent * 2
	}

	if !(params.lineMode == LineModeStaircase || ((minNumberOfPoints == maxNumberOfPoints) && (minNumberOfPoints == 2))) {
		endTime = -1
		for _, res := range results {
			tmp = res.GetStopTime() - res.GetStepTime()
			if endTime == -1 || endTime > tmp {
				endTime = tmp
			}
		}
		timeRange = endTime - startTime
		if timeRange < 0 {
			panic("startTime > endTime!!!")
		}
	}

	//TODO: consolidateDataPoints
	currentXMin := params.area.xmin
	currentXMax := params.area.xmax
	if params.secondYAxis {
		setupTwoYAxes(cr, params, results)
	} else {
		setupYAxis(cr, params, results)
	}

	for currentXMin != params.area.xmin || currentXMax != params.area.xmax {
		currentXMin = params.area.xmin
		currentXMax = params.area.xmax
		if params.secondYAxis {
			setupTwoYAxes(cr, params, results)
		} else {
			setupYAxis(cr, params, results)
		}
	}

	setupXAxis(cr, params, results)

	if !params.hideAxes {
		drawLabels(cr, params, results)
		if !params.hideGrid {
			drawGridLines(cr, params, results)
		}
	}

	drawLines(cr, params, results)
}

func setupTwoYAxes(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	panic("Not Implemented yet")
}

func setupYAxis(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	seriesWithMissingValues := list.New()
	yMin := math.NaN()
	yMax := math.NaN()
	for _, r := range results {
		pushed := false
		for i, v := range r.Values {
			if r.IsAbsent[i] && !pushed {
				seriesWithMissingValues.PushBack(r)
				pushed = true
			} else {
				if math.IsNaN(yMin) || yMin > v {
					yMin = v
				}
				// TODO: Implement 'drawAsInfinite'
				if math.IsNaN(yMax) || yMax < v {
					yMax = v
				}
			}
		}
	}

	if params.areaMode == AreaModeStacked {
		//TODO: https://github.com/brutasse/graphite-api/blob/master/graphite_api/render/glyph.py#L1274
		// Need to implement function that'll sum all results by element and will produce max of it
		panic("Not Implemented yet")
	}

	if yMax < 0 && params.drawNullAsZero && seriesWithMissingValues.Len() > 0 {
		yMax = 0
	}

	// FIXME: Do we really need this check? It should be impossible to meet this conditions
	if math.IsNaN(yMin) {
		yMin = 0
	}
	if math.IsNaN(yMax) {
		yMax = 0
	}

	if !math.IsNaN(params.yMax) {
		yMax = params.yMax
	}
	if !math.IsNaN(params.yMin) {
		yMin = params.yMin
	}

}

func setupXAxis(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	logger.Logln("stubbed setupXAxis()")
}

func drawLabels(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	logger.Logln("stubbed drawLabels()")
}

func drawGridLines(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	logger.Logln("stubbed drawGridLines()")
}

func drawLines(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	logger.Logln("stubbed drawLines()")
}

type SeriesLegend struct {
	name        *string
	color       *string
	secondYAxis bool
}

func drawLegend(cr *cairoSurfaceContext, params *Params, results []*metricData) {
	const (
		padding = 5
	)
	var longestName *string
	var longestNameLen int = -1
	var uniqueNames map[string]bool
	var numRight int = 0
	legend := list.New()
	if params.uniqueLegend {
		uniqueNames = make(map[string]bool)
	}

	for _, res := range results {
		nameLen := len(*(res.Name))
		if longestNameLen == -1 || nameLen > longestNameLen {
			longestNameLen = nameLen
			longestName = res.Name
		}
		if res.secondYAxis {
			numRight += 1
		}
		if params.uniqueLegend {
			if _, ok := uniqueNames[*(res.Name)]; !ok {
				var tmp = SeriesLegend{
					res.Name,
					&res.color,
					res.secondYAxis,
				}
				uniqueNames[*(res.Name)] = true
				legend.PushBack(tmp)
			}
		} else {
			var tmp = SeriesLegend{
				res.Name,
				&res.color,
				res.secondYAxis,
			}
			legend.PushBack(tmp)
		}
	}

	rightSideLabels := false
	testSizeName := *longestName + " " + *longestName
	var textExtents cairo.TextExtents
	cr.context.TextExtents(testSizeName, &textExtents)
	testWidth := textExtents.Width + 2*(params.fontExtents.Height+padding)
	if testWidth+50 < params.width {
		rightSideLabels = true
	}

	cr.context.TextExtents(*longestName, &textExtents)
	boxSize := params.fontExtents.Height - 1
	lineHeight := params.fontExtents.Height + 1
	labelWidth := textExtents.Width + 2*(boxSize+padding)
	cr.context.SetLineWidth(1.0)
	x := params.area.xmin

	if params.secondYAxis && rightSideLabels {
		columns := math.Max(1, math.Floor(math.Floor((params.width-params.area.xmin)/labelWidth)/2.0))
		numberOfLines := math.Max(float64(len(results)-numRight), float64(numRight))
		legendHeight := math.Max(1, (numberOfLines/columns)) * (lineHeight + padding)
		params.area.ymax -= legendHeight
		y := params.area.ymax + (2 * padding)

		xRight := params.area.xmax - params.area.xmin
		yRight := y
		nRight := 0
		n := 0
		for e := legend.Front(); e != nil; e = e.Next() {
			item := e.Value.(SeriesLegend)
			setColor(cr, string2RGBAptr(*item.color))
			if item.secondYAxis {
				nRight += 1
				drawRectangle(cr, params, xRight-padding, yRight, boxSize, boxSize, true)
				color := colors["darkgray"]
				setColor(cr, &color)
				drawRectangle(cr, params, xRight-padding, yRight, boxSize, boxSize, false)
				setColor(cr, &params.fgColor)
				drawText(cr, params, *item.name, xRight-boxSize, yRight, H_ALIGN_RIGHT, V_ALIGN_TOP, 0.0)
				xRight -= labelWidth
				if nRight%int(columns) == 0 {
					xRight = params.area.xmax - params.area.xmin
					yRight += lineHeight
				}
			} else {
				n += 1
				drawRectangle(cr, params, x, y, boxSize, boxSize, true)
				color := colors["darkgray"]
				setColor(cr, &color)
				drawRectangle(cr, params, x, y, boxSize, boxSize, false)
				setColor(cr, &params.fgColor)
				drawText(cr, params, *item.name, x+boxSize+padding, y, H_ALIGN_LEFT, V_ALIGN_TOP, 0.0)
				x += labelWidth
				if n%int(columns) == 0 {
					x = params.area.xmin
					y += lineHeight
				}
			}
		}
		return
	}
	// else
	columns := math.Max(1, math.Floor(params.width/labelWidth))
	numberOfLines := math.Ceil(float64(len(results)) / columns)
	legendHeight := numberOfLines * (lineHeight + padding)
	params.area.ymax -= legendHeight
	y := params.area.ymax + (2 * padding)
	cnt := 0
	for e := legend.Front(); e != nil; e = e.Next() {
		item := e.Value.(SeriesLegend)
		setColor(cr, string2RGBAptr(*item.color))
		if item.secondYAxis {
			drawRectangle(cr, params, x+labelWidth+padding, y, boxSize, boxSize, true)
			color := colors["darkgray"]
			setColor(cr, &color)
			drawRectangle(cr, params, x+labelWidth+padding, y, boxSize, boxSize, false)
			setColor(cr, &params.fgColor)
			drawText(cr, params, *item.name, x+labelWidth, y, H_ALIGN_RIGHT, V_ALIGN_TOP, 0.0)
			x += labelWidth
		} else {
			drawRectangle(cr, params, x, y, boxSize, boxSize, true)
			color := colors["darkgray"]
			setColor(cr, &color)
			drawRectangle(cr, params, x, y, boxSize, boxSize, false)
			setColor(cr, &params.fgColor)
			drawText(cr, params, *item.name, x+boxSize+padding, y, H_ALIGN_LEFT, V_ALIGN_TOP, 0.0)
			x += labelWidth
		}
		if (cnt+1)%int(columns) == 0 {
			x = params.area.xmin
			y += lineHeight
		}
		cnt += 1
	}
	return
}

func drawTitle(cr *cairoSurfaceContext, params *Params) {
	y := params.area.ymin
	x := params.width / 2.0
	lines := strings.Split(params.title, "\n")
	lineHeight := params.fontExtents.Height

	for _, line := range lines {
		drawText(cr, params, line, x, y, H_ALIGN_CENTER, V_ALIGN_TOP, 0.0)
		y += lineHeight
	}
	params.area.ymin = y
	if params.yAxisSide != YAxisSideRight {
		params.area.ymin += float64(params.margin)
	}
}

func drawVTitle(cr *cairoSurfaceContext, params *Params, rightAlign bool) {
	lineHeight := params.fontExtents.Height

	if rightAlign {
		x := params.area.xmax - lineHeight
		y := params.height / 2.0
		for _, line := range strings.Split(params.vtitle, "\n") {
			drawText(cr, params, line, x, y, H_ALIGN_CENTER, V_ALIGN_BASELINE, 90.0)
			x -= lineHeight
		}
		params.area.xmax = x - float64(params.margin) - lineHeight
	} else {
		x := params.area.xmin + lineHeight
		y := params.height / 2.0
		for _, line := range strings.Split(params.vtitle, "\n") {
			drawText(cr, params, line, x, y, H_ALIGN_CENTER, V_ALIGN_BASELINE, 270.0)
			x += lineHeight
		}
		params.area.xmin = x + float64(params.margin) + lineHeight
	}
}

func radians(angle float64) float64 {
	const x = math.Pi / 180
	return angle * x
}

func drawText(cr *cairoSurfaceContext, params *Params, text string, x, y float64, align HAlign, valign VAlign, rotate float64) {
	var h_align, v_align float64
	var textExtents cairo.TextExtents
	var fontExtents cairo.FontExtents
	var origMatrix cairo.Matrix
	cr.context.TextExtents(text, &textExtents)
	cr.context.FontExtents(&fontExtents)

	cr.context.GetMatrix(&origMatrix)
	angle := radians(rotate)
	angle_sin, angle_cos := math.Sincos(angle)

	switch align {
	case H_ALIGN_LEFT:
		h_align = 0.0
	case H_ALIGN_CENTER:
		h_align = textExtents.Width / 2.0
	case H_ALIGN_RIGHT:
		h_align = textExtents.Width
	}
	switch valign {
	case V_ALIGN_TOP:
		v_align = fontExtents.Ascent
	case V_ALIGN_CENTER:
		v_align = fontExtents.Height/2.0 - fontExtents.Descent/2.0
	case V_ALIGN_BOTTOM:
		v_align = -fontExtents.Descent
	case V_ALIGN_BASELINE:
		v_align = 0.0
	}

	cr.context.MoveTo(x, y)
	cr.context.RelMoveTo(angle_sin*(-v_align), angle_cos*v_align)
	cr.context.Rotate(angle)
	cr.context.RelMoveTo(-h_align, 0)
	cr.context.TextPath(text)
	cr.context.Fill()
	cr.context.SetMatrix(&origMatrix)
}

func setColor(cr *cairoSurfaceContext, color *color.RGBA) {
	r, g, b, a := color.RGBA()
	// For some reason, RGBA in Go 1.5 returns 16bit value, even though it's not RGBA64
	cr.context.SetSourceRGBA(float64(r)/65536, float64(g)/65536, float64(b)/65536, float64(a)/65536)
}

func setFont(cr *cairoSurfaceContext, params *Params, size float64) {
	cr.context.SelectFontFace(params.fontName, params.fontItalic, params.fontBold)
	cr.context.SetFontSize(size)
	cr.context.FontExtents(&params.fontExtents)
}

func drawRectangle(cr *cairoSurfaceContext, params *Params, x float64, y float64, w float64, h float64, fill bool) {
	if !fill {
		offset := cr.context.GetLineWidth() / 2.0
		x += offset
		y += offset
		h -= offset
		w -= offset
	}
	cr.context.Rectangle(x, y, w, h-1.0)
	if fill {
		cr.context.Fill()
	} else {
		cr.context.SetDash([]float64{}, 0.0)
		cr.context.Stroke()
	}
}

func string2RGBA(clr string) color.RGBA {
	if c, ok := colors[clr]; ok {
		return c
	}
	return hexToRGBA(clr)
}

func string2RGBAptr(clr string) *color.RGBA {
	c := string2RGBA(clr)
	return &c
}

// https://code.google.com/p/sadbox/source/browse/color/hex.go
// hexToColor converts an Hex string to a RGB triple.
func hexToRGBA(h string) color.RGBA {
	var r, g, b uint8
	if len(h) > 0 && h[0] == '#' {
		h = h[1:]
	}

	if len(h) == 3 {
		h = h[:1] + h[:1] + h[1:2] + h[1:2] + h[2:] + h[2:]
	}

	if len(h) == 6 {
		if rgb, err := strconv.ParseUint(string(h), 16, 32); err == nil {
			r = uint8(rgb >> 16)
			g = uint8((rgb >> 8) & 0xFF)
			b = uint8(rgb & 0xFF)
		}
	}

	return color.RGBA{r, g, b, 255}
}