package main

import (
	"image/color"
	"os"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

var (
	// lcg state
	lcg int
)

func graphScores(records []TeamRecord, darkmode bool) {
	p := plot.New()
	p.X.Label.Text = "Round"
	p.X.Width = 2

	var clr color.Color
	if darkmode {
		clr = color.White
	} else {
		clr = color.Black
	}

	p.X.Label.TextStyle.Color = clr
	p.X.Color = clr
	p.X.Tick.Color = clr
	p.X.Tick.Label.Color = clr
	p.Y.Label.TextStyle.Color = clr
	p.Y.Color = clr
	p.Y.Tick.Color = clr
	p.Y.Tick.Label.Color = clr
	p.Legend.TextStyle.Color = clr
	p.Y.Label.Text = "Score"
	p.Y.Width = 2
	p.BackgroundColor = color.Transparent
	p.BackgroundColor = color.Transparent

	graphData := make([]interface{}, len(records)*2)

	offset := 1.0 / float64(len(records))
	colors := map[uint]RGB{}
	light := 0.45
	if darkmode {
		light = 0.8
	}
	count := uint(1)
	lcg = 37
	for h := 0.0; h < 1; h += offset {
		colors[count] = HSL{
			H: h,
			S: 0.9 + (float64(LCG()%20)-10)/100.0,
			L: light + (float64(LCG()%20)-10)/100.0,
		}.ToRGB()
		count++
	}

	for i, rec := range records {
		graphData[i*2] = rec.Team.Name
		l, _ := plotter.NewLine(getTeamPoints(rec.Team.ID))
		l.LineStyle.Width = vg.Points(2)
		l.LineStyle.Color = color.RGBA{
			R: uint8(float64(0xff) * colors[rec.TeamID].R),
			G: uint8(float64(0xff) * colors[rec.TeamID].G),
			B: uint8(float64(0xff) * colors[rec.TeamID].B),
			A: 255}
		p.Add(l)
		p.Legend.Add(rec.Team.Name, l)
	}

	c := vgimg.PngCanvas{vgimg.NewWith(
		vgimg.UseWH(25*vg.Centimeter, 12*vg.Centimeter),
		vgimg.UseBackgroundColor(color.Transparent),
	)}
	p.Draw(draw.New(c))

	// Save the plot to a png file
	path := "assets/points.png"
	if darkmode {
		path = "assets/points-dark.png"
	}
	f, err := os.Create(path)
	if err != nil {
		errorPrint(err)
		return
	}
	defer f.Close()

	_, err = c.WriteTo(f)
	if err != nil {
		errorPrint(err)
	}
}

// ZX81 <3. Range of 0-100
func LCG() int {
	lcg = (75*lcg + 74) % 101
	return lcg
}

func getTeamPoints(teamID uint) plotter.XYs {
	var records []TeamRecord
	res := db.Where("team_id = ?", teamID).Order("time asc").Find(&records)
	if res.Error != nil {
		errorPrint(res.Error)
		return nil
	}
	pts := make(plotter.XYs, len(records))
	for i := range pts {
		pts[i].X = float64(records[i].Round)
		pts[i].Y = float64(calculateScoreTotal(records[i]))
	}
	return pts
}

/*
	RGB/HSL code from https://github.com/gerow/go-color/blob/master/color.go
*/

type RGB struct {
	R, G, B float64
}

type HSL struct {
	H, S, L float64
}

func hueToRGB(v1, v2, h float64) float64 {
	if h < 0 {
		h += 1
	}
	if h > 1 {
		h -= 1
	}
	switch {
	case 6*h < 1:
		return (v1 + (v2-v1)*6*h)
	case 2*h < 1:
		return v2
	case 3*h < 2:
		return v1 + (v2-v1)*((2.0/3.0)-h)*6
	}
	return v1
}

func (c HSL) ToRGB() RGB {
	h := c.H
	s := c.S
	l := c.L

	if s == 0 {
		// it's gray
		return RGB{l, l, l}
	}

	var v1, v2 float64
	if l < 0.5 {
		v2 = l * (1 + s)
	} else {
		v2 = (l + s) - (s * l)
	}

	v1 = 2*l - v2

	r := hueToRGB(v1, v2, h+(1.0/3.0))
	g := hueToRGB(v1, v2, h)
	b := hueToRGB(v1, v2, h-(1.0/3.0))

	return RGB{r, g, b}
}
